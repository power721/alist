package handles

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

const defaultTimeout = 5000 // Default timeout in milliseconds
const defaultSize = 100
const defaultConcurrency = 5

var (
	appID   = 26375241
	appHash = "70f574f48a016d683c64f2f7a217d04f"
	client  *telegram.Client

	authMu    sync.Mutex
	authState = &AuthState{}
)

type AuthState struct {
	Phone    string
	Code     string
	Password string

	CodeSent         bool
	CodeVerified     bool
	PasswordRequired bool
}

type LinkResponse struct {
	Messages []Message `json:"messages"`
	Total    int       `json:"total"`
	Errors   []string  `json:"errors,omitempty"`
}

type Message struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Channel string `json:"channel"`
	Time    int64  `json:"time"`
}

type restAuthHandler struct{}

func (a restAuthHandler) AcceptTermsOfService(ctx context.Context, _ tg.HelpTermsOfService) error {
	return nil
}

func (a restAuthHandler) Phone(ctx context.Context) (string, error) {
	authMu.Lock()
	defer authMu.Unlock()
	return authState.Phone, nil
}

func (a restAuthHandler) Password(ctx context.Context) (string, error) {
	authMu.Lock()
	authState.PasswordRequired = true
	authMu.Unlock()

	for {
		time.Sleep(200 * time.Millisecond)

		authMu.Lock()
		if authState.Password != "" {
			password := authState.Password
			authMu.Unlock()
			authState.PasswordRequired = false
			return password, nil
		}
		authMu.Unlock()

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
	}
}

func (a restAuthHandler) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	for {
		time.Sleep(200 * time.Millisecond)

		authMu.Lock()
		if authState.Code != "" {
			code := authState.Code
			authMu.Unlock()
			return code, nil
		}
		authMu.Unlock()

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
	}
}

func (a restAuthHandler) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("signup not supported")
}

func SendCodeHandler(c *gin.Context) {
	phone := c.Query("phone")
	if phone == "" {
		c.JSON(400, gin.H{"error": "phone is required"})
		return
	}

	authMu.Lock()
	authState.Phone = phone
	authState.Code = ""
	authState.Password = ""
	authState.CodeSent = true
	authState.CodeVerified = false
	authMu.Unlock()

	go InitTelegramClient()

	c.JSON(200, gin.H{"status": "code sent"})
}

func VerifyCodeHandler(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(400, gin.H{"error": "code is required"})
		return
	}

	authMu.Lock()
	defer authMu.Unlock()
	if !authState.CodeSent {
		c.JSON(400, gin.H{"error": "code not requested"})
		return
	}

	authState.Code = code
	c.JSON(200, gin.H{"status": "code received"})
}

func PasswordHandler(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
	}
	if err := c.BindJSON(&body); err != nil || body.Password == "" {
		c.JSON(400, gin.H{"error": "invalid JSON or empty password"})
		return
	}

	authMu.Lock()
	authState.Password = body.Password
	authMu.Unlock()

	c.JSON(200, gin.H{"status": "password received"})
}

func AuthStatusHandler(c *gin.Context) {
	authMu.Lock()
	defer authMu.Unlock()

	c.JSON(200, gin.H{
		"codeSent":         authState.CodeSent,
		"codeVerified":     authState.CodeVerified,
		"passwordRequired": authState.PasswordRequired,
	})
}

func InitTelegramClient() {
	if client != nil {
		return
	}
	path := "data/session.json"
	if os.Getenv("DOCKER") != "" {
		path = "/data/session.json"
	}
	ctx := context.Background()
	client = telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{
			Path: path,
		},
	})

	flow := auth.NewFlow(
		restAuthHandler{},
		auth.SendCodeOptions{},
	)

	if err := client.Run(ctx, func(ctx context.Context) error {
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return err
		}

		self, err := client.Self(ctx)
		if err != nil {
			return err
		}

		log.Printf("Logged in as %s (%s)", self.Username, self.Phone)

		authMu.Lock()
		authState.CodeVerified = true
		authMu.Unlock()

		<-ctx.Done()
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}

func searchMessageInChannel(ctx context.Context, sender *message.Sender, channelName, query string, size int) ([]Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		peer, err := sender.Resolve(channelName).AsInputPeer(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve channel %s: %w", channelName, err)
		}

		channel, ok := peer.(*tg.InputPeerChannel)
		if !ok {
			return nil, fmt.Errorf("%s is not a channel", channelName)
		}

		api := tg.NewClient(client)
		searchResult, err := api.MessagesSearch(ctx, &tg.MessagesSearchRequest{
			Peer: &tg.InputPeerChannel{
				ChannelID:  channel.ChannelID,
				AccessHash: channel.AccessHash,
			},
			Q:      query,
			Filter: &tg.InputMessagesFilterEmpty{},
			Limit:  size,
		})
		if err != nil {
			return nil, fmt.Errorf("search failed in %s: %w", channelName, err)
		}

		var messages []Message
		switch r := searchResult.(type) {
		case *tg.MessagesChannelMessages:
			for _, msg := range r.Messages {
				if m, ok := msg.(*tg.Message); ok {
					content := m.Message
					if m.Entities != nil {
						for _, entity := range m.Entities {
							if e, ok := entity.(*tg.MessageEntityTextURL); ok {
								content = content + " " + e.URL
							}
						}
					}
					if strings.Contains(content, "http") {
						messages = append(messages, Message{
							ID:      m.ID,
							Content: content,
							Channel: channelName,
							Time:    int64(m.Date),
						})
					}
				}
			}
		default:
			return nil, fmt.Errorf("unexpected result type from %s: %T", channelName, r)
		}

		return messages, nil
	}
}

func parseTimeout(timeoutParam string) int {
	if timeoutParam == "" {
		return defaultTimeout
	}

	timeout, err := strconv.Atoi(timeoutParam)
	if err != nil || timeout <= 500 {
		return defaultTimeout
	}

	return timeout
}

func parseSize(sizeParam string) int {
	if sizeParam == "" {
		return defaultSize
	}

	size, err := strconv.Atoi(sizeParam)
	if err != nil || size < 0 {
		return defaultSize
	}

	return size
}

func gzipMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gz := gzip.NewWriter(w)
			defer gz.Close()

			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Add("Vary", "Accept-Encoding")

			next(&gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
			return
		}

		next(w, r)
	}
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

type SearchRequest struct {
	Keyword         string `json:"keyword"`
	ChannelUsername string `json:"channelUsername"`
	Encode          string `json:"encode"`
	Page            string `json:"page"`
}

func ResolveHandler(c *gin.Context) {
	if client == nil {
		c.JSON(500, gin.H{"error": "Telegram client is nil"})
		return
	}

	username := c.Query("username")
	sender := message.NewSender(tg.NewClient(client))
	log.Printf("resolve username %s", username)

	res, err := sender.Resolve(username).AsInputPeer(c.Request.Context())
	if err != nil {
		log.Printf("failed to resolve %s: %v", username, err)
		c.JSON(400, gin.H{"error": fmt.Sprintf("failed to resolve username: %v", err)})
		return
	}

	c.JSON(200, res)
}

func ValidateHandler(c *gin.Context) {
	if client == nil {
		c.JSON(500, gin.H{"error": "Telegram client is nil"})
		return
	}

	channelsParam := c.Query("channels")
	sender := message.NewSender(tg.NewClient(client))
	log.Printf("validating %s", channelsParam)

	response := validate(c.Request.Context(), sender, channelsParam)
	c.JSON(200, response)
}

func validate(ctx context.Context, sender *message.Sender, channelName string) map[string]bool {
	channels := strings.Split(channelName, ",")
	var results map[string]bool
	results = make(map[string]bool)
	for _, channel := range channels {
		_, err := sender.Resolve(channel).AsInputPeer(ctx)
		results[channel] = err == nil
	}
	return results
}

func SearchHandler(c *gin.Context) {
	if client == nil {
		c.JSON(500, gin.H{"error": "Telegram client is nil"})
		return
	}

	channelsParam := c.Query("channels")
	query := c.Query("query")
	timeoutParam := c.Query("timeout")
	sizeParam := c.Query("size")

	size := parseSize(sizeParam)
	timeoutMs := parseTimeout(timeoutParam)
	timeoutDuration := time.Duration(timeoutMs) * time.Millisecond

	if channelsParam == "" {
		c.JSON(400, gin.H{"error": "channels parameter is required"})
		return
	}

	channels := strings.Split(channelsParam, ",")
	if len(channels) == 0 {
		c.JSON(400, gin.H{"error": "at least one channel must be specified"})
		return
	}

	log.Printf("search %s from channels %s", query, channelsParam)

	ctx, cancel := context.WithTimeout(c.Request.Context(), timeoutDuration)
	defer cancel()

	response := LinkResponse{
		Messages: make([]Message, 0),
		Errors:   make([]string, 0),
	}

	sender := message.NewSender(tg.NewClient(client))
	var wg sync.WaitGroup
	var mu sync.Mutex

	workers := make(chan struct{}, defaultConcurrency)

	for _, channel := range channels {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			continue
		}

		workers <- struct{}{}
		wg.Add(1)
		go func(ch string) {
			defer wg.Done()
			defer func() { <-workers }() // Release worker slot when done

			messages, err := searchMessageInChannel(ctx, sender, ch, query, size)
			if err != nil {
				mu.Lock()
				response.Errors = append(response.Errors, fmt.Sprintf("%s: %v", ch, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			response.Messages = append(response.Messages, messages...)
			mu.Unlock()
		}(channel)
	}

	// Wait for all goroutines to finish or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed
	case <-ctx.Done():
		// Timeout reached
		response.Errors = append(response.Errors, fmt.Sprintf("request timed out after %v", timeoutDuration))
	}

	response.Total = len(response.Messages)
	c.JSON(200, response)
}
