FROM openjdk:17-jdk-slim AS build
WORKDIR /app
COPY ExampleCheck.java .
RUN javac ExampleCheck.java

FROM openjdk:17-jre-slim
WORKDIR /app
COPY --from=build /app/ExampleCheck.class .
ENTRYPOINT ["java", "ExampleCheck"]
