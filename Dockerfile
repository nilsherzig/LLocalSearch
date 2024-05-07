FROM node:alpine
ARG PUBLIC_VERSION="0"
ENV PUBLIC_VERSION=${PUBLIC_VERSION}

WORKDIR /app
COPY package.json ./
RUN npm install

COPY . .
RUN npm run build
RUN npm prune --omit=dev

CMD ["node", "custom-server.js"]

EXPOSE 3000
