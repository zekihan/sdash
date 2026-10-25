package api

import "testing"

func redirectTestCases() []struct{ target, want string } {
	return []struct{ target, want string }{
		{"", "/"},
		{"/", "/"},
		{"/dashboard", "/dashboard"},
		{"/dashboard/group/a?one=1&two=2#section", "/dashboard/group/a?one=1&two=2#section"},
		{
			"/search?q=https%3A%2F%2Fexample.com&next=%2Fdashboard",
			"/search?q=https%3A%2F%2Fexample.com&next=%2Fdashboard",
		},
		{"/group/hello%20world?tag=a%26b", "/group/hello%20world?tag=a%26b"},
		{"/100%25", "/100%25"},
		{"relative", "/"},
		{"https://external.example", "/"},
		{"//external.example", "/"},
		{"///external.example", "/"},
		{`/\external.example`, "/"},
		{`\external.example`, "/"},
		{"/%2fexternal.example", "/"},
		{"/%2Fexternal.example", "/"},
		{"/%5cexternal.example", "/"},
		{"/%252fexternal.example", "/"},
		{"/%255Cexternal.example", "/"},
		{"%2f%2fexternal.example", "/"},
		{"/safe%0d%0aLocation:evil", "/"},
		{"/\t/external.example", "/"},
		{"/%09/external.example", "/"},
		{"/%250a/external.example", "/"},
		{"/bad%zz", "/"},
	}
}

func TestLocalRedirectPath(t *testing.T) {
	t.Parallel()

	for _, tc := range redirectTestCases() {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			if got := localRedirectPath(tc.target); got != tc.want {
				t.Errorf("localRedirectPath(%q) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}
}
