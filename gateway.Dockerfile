FROM openresty/openresty:alpine

RUN apk add perl
RUN apk add curl
RUN opm get ledgetech/lua-resty-http \
    && opm get cdbattags/lua-resty-jwt \ 
    && opm get openresty/lua-resty-redis
RUN opm get fffonion/lua-resty-openssl

COPY ./nginx.conf /usr/local/openresty/nginx/conf/nginx.conf


CMD ["/usr/local/openresty/bin/openresty", "-g", "daemon off;"]