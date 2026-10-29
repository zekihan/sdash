package views

import _ "github.com/a-h/templ" // Import templ to ensure it is initialized

func Dummy() {
	// This is a dummy function to make sure the package is not empty
	// and it is not removed by the compiler.
	// It is not used in the codebase.
}

func iconInitial(title string) string {
	for _, initial := range title {
		return string(initial)
	}

	return ""
}
