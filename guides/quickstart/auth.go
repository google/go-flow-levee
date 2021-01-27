package auth

import "log"

type Authentication struct {
	Username string
	Password string `datapolicy:"password"`
}

func authenticate(auth Authentication) (*AuthenticationResponse, error) {
	response, err := makeAuthenticatedRequest(auth)
	if err != nil {
		log.Printf("unable to make authenticated request: incorrect authentication? %v", auth)
		return nil, err
	}
	return response, nil
}

// just a stub, to allow the code to compile
type AuthenticationResponse struct{}

// just a stub, to allow the code to compile
func makeAuthenticatedRequest(Authentication) (*AuthenticationResponse, error) { return nil, nil }
