FROM node:20-alpine
WORKDIR /app
COPY package.json ./
RUN npm install --production
COPY check.js ./
CMD ["node", "check.js"]
