package cmd

import (
	"strings"
	"testing"
)

func TestLocalSDKRewrite(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		data, err := initTemplates.ReadFile("templates/init/" + path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	cases := []struct {
		name string
		sdk  *localSDK
		tmpl string
		rel  string
		want string
		gone string
	}{
		{
			name: "next",
			sdk: &localSDK{framework: "next", root: "/src/lumo", npmDeps: [][2]string{
				{"@lumoauth/nextjs", "file:./vendor/lumoauth-nextjs-1.0.0.tgz"},
				{"@lumoauth/shared", "file:./vendor/lumoauth-shared-1.0.0.tgz"},
			}},
			tmpl: "next/package.json", rel: "package.json",
			want: `"@lumoauth/shared": "file:./vendor/lumoauth-shared-1.0.0.tgz",`,
			gone: `"@lumoauth/nextjs": "^1.0.0"`,
		},
		{
			name: "express",
			sdk: &localSDK{framework: "express", root: "/src/lumo", npmDeps: [][2]string{
				{"@lumoauth/express", "file:./vendor/lumoauth-express-1.0.0.tgz"},
			}},
			tmpl: "express/package.json", rel: "package.json",
			want: `"@lumoauth/express": "file:./vendor/lumoauth-express-1.0.0.tgz"`,
			gone: `"@lumoauth/express": "^1.0.0"`,
		},
		{
			name: "fastapi",
			sdk:  &localSDK{framework: "fastapi", root: "/src/lumo"},
			tmpl: "fastapi/requirements.txt", rel: "requirements.txt",
			want: "lumoauth[fastapi] @ file:///src/lumo/sdk-python",
			gone: "lumoauth[fastapi]>=",
		},
		{
			name: "go",
			sdk:  &localSDK{framework: "go", root: "/src/lumo"},
			tmpl: "go/go.mod.tmpl", rel: "go.mod",
			want: "replace github.com/lumoauth/lumo-auth-go => /src/lumo/sdk-go",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.sdk.rewrite(tc.rel, read(tc.tmpl))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("missing %q in:\n%s", tc.want, got)
			}
			if tc.gone != "" && strings.Contains(got, tc.gone) {
				t.Errorf("registry dependency %q survived in:\n%s", tc.gone, got)
			}
		})
	}
}
