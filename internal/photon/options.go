package photon

type PhotonServerOption func(*PhotonServer)

func WithArgs(args []string) PhotonServerOption {
	return func(s *PhotonServer) {
		s.additionalArgs = args
	}
}
