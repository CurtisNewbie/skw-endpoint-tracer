# skw-endpoint-tracer

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
