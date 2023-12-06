package stream

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/net"
	"github.com/alist-org/alist/v3/pkg/http_range"
	log "github.com/sirupsen/logrus"
)

func GetRangeReadCloserFromLink(size int64, link *model.Link) (model.RangeReadCloserIF, error) {
	if len(link.URL) == 0 {
		return nil, fmt.Errorf("can't create RangeReadCloser since URL is empty in link")
	}
	//remoteClosers := utils.EmptyClosers()
	rangeReaderFunc := func(ctx context.Context, r http_range.Range) (io.ReadCloser, error) {
		if link.Concurrency != 0 || link.PartSize != 0 {
			header := net.ProcessHeader(http.Header{}, link.Header)
			down := net.NewDownloader(func(d *net.Downloader) {
				d.Concurrency = link.Concurrency
				d.PartSize = link.PartSize
			})
			req := &net.HttpRequestParams{
				URL:       link.URL,
				Range:     r,
				Size:      size,
				HeaderRef: header,
			}
			rc, err := down.Download(ctx, req)
			if err != nil {
				return nil, errs.NewErr(err, "GetReadCloserFromLink failed")
			}
			return rc, nil

		}
		if len(link.URL) > 0 {
			response, err := RequestRangedHttp(ctx, link, r.Start, r.Length)
			if err != nil {
				if response == nil {
					return nil, fmt.Errorf("http request failure, err:%s", err)
				}
				return nil, fmt.Errorf("http request failure,status: %d err:%s", response.StatusCode, err)
			}
			if r.Start == 0 && (r.Length == -1 || r.Length == size) || response.StatusCode == http.StatusPartialContent ||
				checkContentRange(&response.Header, r.Start) {
				return response.Body, nil
			} else if response.StatusCode == http.StatusOK {
				log.Warnf("remote http server not supporting range request, expect low perfromace!")
				readCloser, err := net.GetRangedHttpReader(response.Body, r.Start, r.Length)
				if err != nil {
					return nil, err
				}
				return readCloser, nil

			}

			return response.Body, nil
		}

		return nil, errs.NotSupport
	}
	resultRangeReadCloser := model.RangeReadCloser{RangeReader: rangeReaderFunc}
	return &resultRangeReadCloser, nil
}

func RequestRangedHttp(ctx context.Context, link *model.Link, offset, length int64) (*http.Response, error) {
	header := net.ProcessHeader(http.Header{}, link.Header)
	header = http_range.ApplyRangeToHttpHeader(http_range.Range{Start: offset, Length: length}, header)

	return net.RequestHttp(ctx, "GET", header, link.URL)
}

// 139 cloud does not properly return 206 http status code, add a hack here
func checkContentRange(header *http.Header, offset int64) bool {
	start, _, err := http_range.ParseContentRange(header.Get("Content-Range"))
	if err != nil {
		log.Warnf("exception trying to parse Content-Range, will ignore,err=%s", err)
	}
	if start == offset {
		return true
	}
	return false
}
