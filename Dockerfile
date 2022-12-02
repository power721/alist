FROM xiaoyaliu/alist

LABEL MAINTAINER="Har01d"

VOLUME /opt/alist/data/

WORKDIR /opt/alist/

COPY entrypoint.sh /entrypoint.sh
COPY updateindex /updateindex
COPY search-api /www/cgi-bin
COPY nginx.conf /etc/nginx/http.d/default.conf

EXPOSE 80
EXPOSE 5244

ENTRYPOINT [ "/entrypoint.sh" ]

CMD [ "./alist", "server", "--no-prefix" ]
