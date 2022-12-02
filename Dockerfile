FROM xiaoyaliu/alist

LABEL MAINTAINER="Har01d"

VOLUME /opt/alist/data/

WORKDIR /opt/alist/

COPY entrypoint.sh updatedb updateindex /
COPY search search-api /www/cgi-bin/
COPY data.db version.txt /opt/alist/data/
COPY nginx.conf /etc/nginx/http.d/default.conf

EXPOSE 80

ENTRYPOINT [ "/entrypoint.sh" ]

CMD [ "./alist", "server", "--no-prefix" ]
