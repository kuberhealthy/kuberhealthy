FROM node 
COPY package*.json ./
RUN npm install @kubernetes/client-node
RUN npm install

COPY . .
ENTRYPOINT ["node", "index.js"]