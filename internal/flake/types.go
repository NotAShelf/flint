package flake

type Relations struct {
	Deps        map[string][]string
	ReverseDeps map[string][]string
}

type Input struct {
	Type  string
	Owner string
	Repo  string
	Host  string
	URL   string
	Path  string
}
