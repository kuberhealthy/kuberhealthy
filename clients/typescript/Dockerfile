FROM node:18-alpine
WORKDIR /app
COPY package*.json tsconfig.json ./
RUN npm install
COPY src/ src/
RUN npm run build
CMD ["node", "dist/example.js"]
