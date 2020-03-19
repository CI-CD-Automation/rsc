package ssc // import "gopkg.in/rightscale/rsc.v8/ss/ssc"

import "gopkg.in/rightscale/rsc.v8/rsapi"

// API is the Self-Service catalog client.
type API struct {
	*rsapi.API
}

// New returns a Self-Service catalog API client.
// It makes a test API request and returns an error if authentication fails.
func New(h string, a rsapi.Authenticator) *API {
	api := rsapi.New(h, a)
	api.Metadata = GenMetadata
	ssAPI := API{API: api}
	return &ssAPI
}
