package client

import (
	"context"
	"strings"

	"github.com/curtisnewbie/miso/util"
	"github.com/machinebox/graphql"
	api "skywalking.apache.org/repo/goapi/query"
)

var (
	FilteredNodeNames   = util.NewSet[string]()
	Headers             = map[string]string{}
	Debug               = false
	Path                = "https://localhost:8080/graphql"
	SearchEndpointLimit = 1500
)

var (
	cache = map[string][]*api.EndpointNode{}
)

func init() {
	FilteredNodeNames.AddAll([]string{
		"UndertowDispatch", "SpringAsync", "Kafka/Producer/Callback",
	})
}

type MergePrefixMap map[string] /* service */ map[string] /* path-prefix */ string /* name to be used */

func PrintEndpointRoutes(dependencies map[string][]*Endpoint) {
	for s, roots := range dependencies {
		util.Printlnf("\n# Routes to Service %v\n", s)
		util.Printlnf("```")
		for _, ep := range roots {
			util.Printlnf("%v (%v)", s, ep.Name)
			for _, d := range ep.Child {
				util.Printlnf("\t<- %v (%v)", d.ServiceName, d.Name)
				for _, dd := range d.Child {
					util.Printlnf("\t\t<- %v (%v)", dd.ServiceName, dd.Name)
				}
			}
		}
		util.Printlnf("```")
	}
}

func PullEndpointRoutes(serviceNames []string, dur Duration,
	mergePrefixMap MergePrefixMap) map[string][]*Endpoint {

	if dur.Step == "" {
		dur.Step = "DAY"
	}

	svcNameSet := util.NewSet[string]()
	svcNameSet.AddAll(serviceNames)

	services := Services(dur)
	services = util.Filter(services, func(s Service) bool { return svcNameSet.Has(s.Name) })

	aggregate := map[string][]*Endpoint{}
	mergedEndpoints := map[string]*Endpoint{}
	addChild := func(r *Endpoint, curr *Endpoint) *Endpoint {
		if prev, ok := r.Child[curr.Id]; ok {
			return prev
		}
		r.Child[curr.Id] = curr
		return curr
	}
	addEndpoint := func(service string, v *Endpoint) {
		aggregate[service] = append(aggregate[service], v)
	}
	newEndpoint := func(service string, id string, name string, merged bool) (*Endpoint, bool) {
		if merged {
			if prev, ok := mergedEndpoints[service+name]; ok {
				prev.Merged = true
				return prev, true
			}
		}
		v := &Endpoint{Id: id, Name: name, Child: map[string]*Endpoint{}, ServiceName: service}
		if merged {
			mergedEndpoints[service+name] = v
		}
		return v, false
	}

	for _, s := range services {

		endpoints := SearchEndpoints(s.ID, "", SearchEndpointLimit)

		for _, ep := range endpoints {
			if strings.HasPrefix(ep.Name, "Hystrix/") || !filterNode(ep.Name) {
				continue
			}

			origin := ep.Name
			n, merged := mergeDuplicate(s.Name, ep.Name, mergePrefixMap)
			if merged {
				ep.Name = n
			}

			p, isPrev := newEndpoint(s.Name, ep.ID, ep.Name, merged)
			if !isPrev {
				addEndpoint(s.Name, p)
			}

			if Debug {
				util.Printlnf("[debug] %v (%v)", s.Name, ep.Name)
			}

			ed := EndpointFromDependency(ep.ID, dur)

			if Debug {
				util.Printlnf("[debug] %v (%v), ed: %#v", s.Name, origin, ed)
			}

			for _, v := range ed {

				if !filterNode(v.Name) {
					continue
				}

				vp, _ := newEndpoint(v.ServiceName, v.ID, v.Name, false)
				vp = addChild(p, vp)

				if Debug {
					if merged {
						mep := mergedEndpoints[s.Name+ep.Name]
						if mep != nil {
							util.Printlnf("[debug] merging, %v (%v), %#v", s.Name, ep.Name, *mep)
						} else {
							util.Printlnf("[debug] merging, %v (%v), nil", s.Name, ep.Name)
						}
					}
				}

				if Debug {
					util.Printlnf("[debug]\t<- %v (%v)", v.ServiceName, v.Name)
				}

				edd := EndpointFromDependency(v.ID, dur)

				for _, vv := range edd {
					if Debug {
						util.Printlnf("[debug]\t\t<- %v (%v)", vv.ServiceName, vv.Name)
					}
					vvp, _ := newEndpoint(vv.ServiceName, vv.ID, vv.Name, false)
					addChild(vp, vvp)
				}
			}
		}
	}
	return aggregate
}

func filterNode(name string) bool {
	return !FilteredNodeNames.Has(name)
}

func mergeDuplicate(serviceName string, name string, mergePrefixMap MergePrefixMap) (string, bool) {
	if m, ok := mergePrefixMap[serviceName]; ok {
		for k, v := range m {
			if strings.HasPrefix(name, k) {
				name = v
				if name == "" {
					name = k
				}
				return name, true

			}
		}
	}
	return name, false
}

func SearchEndpoints(serviceID string, keyword string, limit int) []api.Endpoint {
	var response map[string][]api.Endpoint
	var request *graphql.Request = graphql.NewRequest(`
		query ($keyword: String!, $serviceId: ID!, $limit: Int!) {
			result: findEndpoint(keyword: $keyword, serviceId: $serviceId, limit: $limit) {
				id name
			}
		}
	`)
	request.Var("serviceId", serviceID)
	request.Var("keyword", keyword)
	request.Var("limit", limit)
	executeQuery(request, &response)
	return response["result"]
}

func Services(duration Duration) []Service {
	var response map[string][]Service
	request := graphql.NewRequest(`
		query ($duration: Duration!) {
			services: getAllServices(duration: $duration) {
				id name
			}
		}
	`)
	request.Var("duration", duration)
	executeQuery(request, &response)
	return response["services"]
}

func newClient() (client *graphql.Client) {
	client = graphql.NewClient(Path)
	client.Log = func(msg string) {
		if Debug {
			util.Printlnf("[debug] client: %v", msg)
		}
	}
	return
}

func executeQuery(request *graphql.Request, response interface{}) {
	for k, v := range Headers {
		request.Header.Set(k, v)
	}

	client := newClient()
	ctx := context.Background()
	err := client.Run(ctx, request, response)
	util.Must(err)
}

func EndpointDependency(endpointID string, duration Duration) api.EndpointTopology {
	var response map[string]api.EndpointTopology

	request := graphql.NewRequest(`
		query ($endpointId:ID!, $duration: Duration!) {
			result: getEndpointDependencies(duration: $duration, endpointId: $endpointId) {
				nodes {
					id
					name
					serviceId
					serviceName
					type
					isReal
				}
				calls {
					id
					source
					target
					detectPoints
					sourceComponents
					targetComponents
				}
			}
		}`,
	)
	request.Var("endpointId", endpointID)
	request.Var("duration", duration)

	executeQuery(request, &response)

	return response["result"]
}

func EndpointFromDependency(endpointId string, duration Duration) []*api.EndpointNode {
	if v, ok := cache[endpointId]; ok {
		return v
	}

	ed := EndpointDependency(endpointId, duration)
	recv := util.NewSet[string]()
	for _, c := range ed.Calls {
		if c.Target == endpointId {
			recv.Add(c.Source)
		}
	}
	ed.Nodes = util.CopyFilter(ed.Nodes, func(n *api.EndpointNode) bool {
		return n.IsReal && recv.Has(n.ID)
	})
	cache[endpointId] = ed.Nodes
	return ed.Nodes
}

type Endpoint struct {
	// Endpoint Id.
	//
	// If the endpoint is merged with another endpoint (based on common prefix),
	// the endpoint_id presented is not accurate anymore.
	Id          string
	Name        string
	ServiceName string
	Merged      bool
	Child       map[string]*Endpoint
}

type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Duration struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Step  string `json:"step"`
}
