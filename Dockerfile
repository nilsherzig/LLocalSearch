FROM node:alpine as build
# ENV NODE_ENV=production
ARG PUBLIC_VERSION="0"
ENV PUBLIC_VERSION=${PUBLIC_VERSION}

WORKDIR /app
COPY package.json ./
RUN npm install

COPY . .
RUN npm run build
RUN npm prune 

FROM node:alpine 

COPY --from=build /app/build /app/build
COPY --from=build /app/node_modules /app/node_modules
COPY --from=build /app/custom-server.js /app/custom-server.js

WORKDIR /app

CMD ["node", "custom-server.js"]

EXPOSE 3000

