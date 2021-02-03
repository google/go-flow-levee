// full path: github.com/google/go-flow-levee/guides/quickstart
package quickstart

import "log"

type Authentication struct {
	Username string
	Password string `datapolicy:"password"`
}

func authenticate(auth Authentication) (*AuthenticationResponse, error) {
	response, err := makeAuthenticationRequest(auth)
	if err != nil {
		log.Printf("unable to make authenticated request: incorrect authentication? %v", auth)
		return nil, err
	}
	return response, nil
}

// just a stub, to allow the code to compile
type AuthenticationResponse struct{}

// just a stub, to allow the code to compile
func makeAuthenticationRequest(Authentication) (*AuthenticationResponse, error) { return nil, nil }

//lint:file-ignore U1000 ignore unused functions
