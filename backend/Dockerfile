FROM alpine:3.21

WORKDIR /app
RUN adduser -D -u 10001 appuser

COPY server ./server
RUN chmod +x ./server
RUN mkdir -p /app/uploads && chown -R appuser:appuser /app

USER appuser
EXPOSE 8080

CMD ["./server"]
