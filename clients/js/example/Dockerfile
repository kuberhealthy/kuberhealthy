FROM node 
COPY package*.json ./
RUN npm install

COPY . .
ENTRYPOINT ["node", "check.js"]