# skw-endpoint-tracer

- https://skywalking.apache.org/docs/main/v10.0.0/en/api/query-protocol/#logs
- https://github.com/apache/skywalking-query-protocol
- https://github.com/apache/skywalking-cli

```go
func main() {
	client.Path = "https://my-skywalking-server/graphql"
	client.Headers = map[string]string{
	}
	dur := client.Duration{Start: "2024-08-15", End: "2024-08-16"}
	mergedPrefix := client.MergePrefixMap{
		"myapp": {
			"/app/id/": "/app/id", // e.g., /app/id/1, /app/id/2 ...
		},
	}
	services := []string{
		"myapp",
	}
	routes := client.PullEndpointRoutes(services, dur, mergedPrefix)
	client.PrintEndpointRoutes(routes)
}
```
