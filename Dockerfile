# FROM node:alpine as build
# # ENV NODE_ENV=production
# ARG PUBLIC_VERSION="0"
# ENV PUBLIC_VERSION=${PUBLIC_VERSION}
#
# WORKDIR /app
# COPY package.json ./
# RUN npm install
#
# COPY . .
# RUN npm run build
# RUN npm prune 
#
# FROM node:alpine 
#
# COPY --from=build /app/build /app/build
# COPY --from=build /app/node_modules /app/node_modules
# COPY --from=build /app/custom-server.js /app/custom-server.js
#
# WORKDIR /app
#
# CMD ["node", "custom-server.js"]
#
# EXPOSE 3000
FROM node:alpine as build
ARG PUBLIC_VERSION="0"
ENV PUBLIC_VERSION=${PUBLIC_VERSION}

ADD . /app
WORKDIR /app

RUN npm install
RUN npm run build

FROM nginx:stable
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build /usr/share/nginx/html
