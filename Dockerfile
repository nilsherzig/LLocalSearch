FROM node:alpine as build
ARG PUBLIC_VERSION="0"
ENV PUBLIC_VERSION=${PUBLIC_VERSION}

ADD . /app
WORKDIR /app

RUN npm install
RUN npm run build

FROM nginx:stable-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build /usr/share/nginx/html
