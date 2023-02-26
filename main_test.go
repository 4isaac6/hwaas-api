package main

import (
    "io"
    "os"
    "fmt"
    "time"
    "context"
    "strings"
    "testing"
    "net/http"
    "encoding/json"
    "net/http/httptest"

    "golang.org/x/oauth2"
    "github.com/spf13/viper"
    "github.com/google/go-github/v49/github"
)

var client *github.Client

func setup() (err error) {
    ctx = context.Background()

    // Load configuration files.
    viper.AddConfigPath("config")
    viper.SetConfigName("env")
    viper.MergeInConfig()
    viper.SetConfigName("testing")
    viper.MergeInConfig()

    // Create GitHub API client.
    t := strings.Replace(viper.GetString("user.token"), "Bearer", "", 1)
    ts := oauth2.StaticTokenSource(
        &oauth2.Token{AccessToken: t},
    )
    tc := oauth2.NewClient(ctx, ts)
    client = github.NewClient(tc)

    return err
}

func shutdown() (err error) {
    return
}

func TestMain(m *testing.M) {
    if err := setup(); err != nil {
        fmt.Printf("Error in setup: %v\n", err)
        os.Exit(2)
    }

    code := m.Run()

    if err := shutdown(); err != nil {
        fmt.Printf("Error in shutdown: %v\n", err)
        os.Exit(2)
    }

    os.Exit(code)
}

// -- TESTS --

func TestAuthorize(t *testing.T) {
    var authorizeTestCases = []authorizeTestCase{
        {
            testName:   "A valid token should return an authorized client",
            token:      strings.Replace(viper.GetString("user.token"), "Bearer", "", 1),
            expected:   true,
        },
        {
            testName:   "An invalid token should not return an authorized client",
            token:      "",
            expected:   false,
        },
    }

    for _, c := range authorizeTestCases {
        t.Run(c.testName, func(t *testing.T) {
            assertAuthorize(t, c.token, c.expected)
        })
    }
}

func TestFindLanguage(t *testing.T) {
    var findLanguageTestCases = []findLanguageTestCase{
        {
            testName:   "A language properly named should find a language's file",
            language:   "go",
            expected:   "g/Go.go",
        },
        {
            testName:   "A language improperly named should not find a language's file",
            language:   "notalang",
            expected:   "", // `nil` pointer to expected repository content
        },
    }

    // For simplicity, all the test cases should be found (or not) in the "g" subdirectory of the repository.
    _, rcs, _, _ := client.Repositories.GetContents(
        ctx,
        viper.GetString("repository.user"),
        viper.GetString("repository.name"),
        "g",
        nil,
    )

    var rc *github.RepositoryContent
    for _, c := range findLanguageTestCases {
        if c.expected == "" {
            rc = nil
        } else {
            rc, _, _, _ = client.Repositories.GetContents(
                ctx,
                viper.GetString("repository.user"),
                viper.GetString("repository.name"),
                c.expected,
                nil,
            )
        }

        t.Run(c.testName, func(t *testing.T) {
            assertFindLanguage(t, rcs, c.language, rc)
        })
    }
}

func TestFindLanguages (t *testing.T) {
    var findLanguagesTestCases = []findLanguagesTestCase{
        {
            testName:   "A language formatted improperly should not be found",
            string:     `
                this is not a language
            `,
            expected:   []*Language{},
        },
        {
            testName:   "A language formatted properly without an extension should be found",
            string:     `
                -- some improperly formatted garbage --
                * [lang](l/lang)
                -- some improperly formatted garbage --
            `,
            expected:   []*Language{
                &Language{
                    Name: "lang",
                    Extension: "",
                },
            },
        },
        {
            testName:   "A language formatted properly with an extension should be found",
            string:     `
                -- some improperly formatted garbage --
                * [lang](l/lang.ext)
                -- some improperly formatted garbage --
            `,
            expected:   []*Language{
                &Language{
                    Name: "lang",
                    Extension: ".ext",
                },
            },
        },
        {
            testName:   "Multiple languages formatted properly should be found",
            string:     `
                -- some improperly formatted garbage --
                * [lang1](l/lang1.ext)
                -- some improperly formatted garbage --
                * [lang2](l/lang2.ext)
                -- some improperly formatted garbage --
            `,
            expected:   []*Language{
                &Language{
                    Name: "lang1",
                    Extension: ".ext",
                },
                &Language{
                    Name: "lang2",
                    Extension: ".ext",
                },
            },
        },
        {
            testName:   "A language formatted properly prefixed with a hash (#) character should be not found",
            string:     `
                -- some improperly formatted garbage --
                * [lang](#/lang.ext)
                -- some improperly formatted garbage --
            `,
            expected:   []*Language{},
        },
        {
            testName:   "A language formatted properly prefixed with a URL-encoded hash (%23) character should be not found",
            string:     `
                -- some improperly formatted garbage --
                * [lang](%23/lang.ext)
                -- some improperly formatted garbage --
            `,
            expected:   []*Language{
                &Language{
                    Name: "lang",
                    Extension: ".ext",
                },
            },
        },
        {
            testName:   "A language with an invalid URL encoding should not be found and not cause ",
            string:     `
                -- some improperly formatted garbage --
                * [lang](l/lang.ext)
                -- some improperly formatted garbage --
                * [invalid](%23/invalid%)
                -- some improperly formatted garbage --
            `,
            expected:   []*Language{
                &Language{
                    Name: "lang",
                    Extension: ".ext",
                },
            },
        },
    }

    for _, c := range findLanguagesTestCases {
        t.Run(c.testName, func(t *testing.T) {
            assertFindLanguages(t, c.string, c.expected)
        })
    }
}

func TestIsLanguage(t *testing.T) {
    var isLanguageTestCases = []isLanguageTestCase{
        // Common language.
        {
            testName:   "A language without special characters should be a language",
            language:   "Go",
            path:       "g/Go.go",
            expected:   true,
		},

        // Language with a dot character in the name.
        {
            testName:	"A language with a dot character should be a language",
            language:   "Node.js",
            path:       "n/Node.js.js",
            expected:	true,
		},

        // Languages with special characters as the names.
        {
            testName:	"A language with Arabic characters should be a language",
            language:	"قلب",
            path:		"#/قلب",
            expected:	true,
		},
        {
            testName:	"A language with Chinese characters should be a language",
            language:	"火星文",
            path:		"#/火星文.martian",
            expected:	true,
        },
        {
            testName:	"A language with Greek characters should be a language",
            language:	"μλ",
            path:		"#/μλ",
            expected:	true,
        },
        {
            testName:	"A language with Japanese characters should be a language",
            language:	"なでしこ",
            path:		"#/なでしこ.nako",
            expected:	true,
        },
        {
            testName:	"A language with Runic characters should be a language",
            language:	"ᚱᚢᚾᛅᛦ",
            path:		"#/ᚱᚢᚾᛅᛦ",
            expected:	true,
		},
        {
            testName:	"A language with punctuation characters should be a language",
            language:	"!@#$%^&∗()_+",
            path:		"#/!@#$%^&∗()_+",
            expected:	true,
        },
        {
            testName:	"A language with special characters should be a language",
            language:	"∗﹥﹤﹥",
            path:		"#/∗﹥﹤﹥",
            expected:	true,
        },
        {
            testName:	"A language with emojis should be a language",
            language:	"🆒",
            path:		"#/🆒",
            expected:	true,
		},

        // Expected failures.
        {
            testName:	"A language with a mismatched, but valid, file path should not be a language",
            language:	"notalang",
            path:		"g/Go.go",
            expected:   false,
        },
    }

    var rc *github.RepositoryContent
    for _, c := range isLanguageTestCases {
        rc, _, _, _ = client.Repositories.GetContents(
            ctx,
            viper.GetString("repository.user"),
            viper.GetString("repository.name"),
            c.path,
            nil,
        )

        t.Run(c.testName, func(t *testing.T) {
            assertIsLanguage(t, rc, c.language, c.expected)
        })
    }
}

func TestLoadConfigs (t *testing.T) {
    var loadConfigsTestCases = []loadConfigsTestCase{
        {
            testName:   "A config that exists should load",
            names:      []string{"testing"},
            expected:   true,
        },
        {
            testName:   "A config that does not exists should not load",
            names:      []string{"notafile"},
            expected:   false,
        },
        {
            testName:   "Many configs that all exist should load",
            names:      []string{"testing", "env"},
            expected:   true,
        },
        {
            testName:   "Many configs that do not all exist should not load",
            names:      []string{"testing", "notafile"},
            expected:   false,
        },
    }

    for _, c := range loadConfigsTestCases {
        t.Run(c.testName, func(t *testing.T) {
            assertLoadConfigs(t, c.names, c.expected)
        })
    }
}

func TestHome(t *testing.T) {
    var homeTestCases = []routeTestCase{
        {
            testName:   "The 'home' route should return a welcome message",
            path:       "/api",
            handler:    home,
            expected:   struct { Message string } {
                Message: "Welcome to the Hello World API!",
            },
        },
    }

    for _, c := range homeTestCases {
        t.Run(c.testName, func(t *testing.T) {
            body := assertRoute(t, c.handler, c.path)
            assertHome(t, body, c.expected)
        })
    }
}

func TestGetLanguages(t *testing.T) {
    var getLanguagesTestCases = []routeTestCase{
        {
            testName:   "The 'languages' route should return a instance of the LanguagesResponse struct",
            path:       "/api/languages",
            handler:    getLanguages,
            expected:   &LanguagesResponse {
                Languages:      []*Language {},
                RequestedAt:    time.Now(),
            },
        },
    }

    for _, c := range getLanguagesTestCases {
        t.Run(c.testName, func(t *testing.T) {
            body := assertRoute(t, c.handler, c.path)
            assertGetLanguages(t, body, c.expected)
        })
    }
}

func TestGetLanguage(t *testing.T) {
    var getLanguageTestCases = []routeTestCase{
        {
            testName:   "The 'language' route should return a instance of the LanguageResponse struct",
            // A non-alphabetic character language to use the '#' directory.
            path:       "/api/language/!",
            handler:    getLanguage,
            expected:   &LanguageResponse {
                Code:           &Code {},
                Language:       &Language {},
                RequestedAt:    time.Now(),
            },
        },
    }

    for _, c := range getLanguageTestCases {
        t.Run(c.testName, func(t *testing.T) {
            body := assertRoute(t, c.handler, c.path)
            assertGetLanguage(t, body, c.expected)
        })
    }
}

// --- ASSERTS ---

func assertAuthorize(t *testing.T, s string, expected bool) {
    client := authorize(s)
    _, _, err := client.Users.Get(ctx, "")

    if expected && err != nil {
        t.Errorf("Token (%v) did not return an authorized GitHub API client, unexpectedly", s)
    } else if !expected && err == nil {
        t.Errorf("Token (%v) did return an authorized GitHub API client, unexpectedly", s)
    }
}

func assertFindLanguage(t *testing.T, rcs []*github.RepositoryContent, s string, expected *github.RepositoryContent) {
    res, err := findLanguage(rcs, s)

    if err != nil && expected != nil {
        t.Errorf("Language (%v) was not found (%v), but was expected (%v)", s, nil, expected.GetName())
    } else if err == nil && expected == nil {
        t.Errorf("Language (%v) was found (%v), but was not expected (%v)", s, res.Name, nil)
    }
}

func assertFindLanguages(t *testing.T, s string, expected []*Language) {
    res := findLanguages(s)

    if len(res) != len(expected) {
        t.Errorf("Result (%v) has a different length than expected (%v)", len(res), len(expected))
    }

    for i, r := range res {
        e := expected[i]
        if r.Name != e.Name {
            t.Errorf("Result (%v) has a different name than expected (%v)", r.Name, e.Name)
        }

        if r.Extension != e.Extension {
            t.Errorf("Result (%v) has a different extension than expected (%v)", r.Extension, e.Extension)
        }
    }
}

func assertIsLanguage(t *testing.T, rc *github.RepositoryContent, language string, expected bool) {
    if isLanguage(rc, language) != expected {
        if expected {
            t.Errorf("`%s` expected to be a language", language)
        } else {
            t.Errorf("`%s` expected not to be a language", language)
        }
    }
}

func assertLoadConfigs(t *testing.T, names []string, expected bool) {
    err := loadConfigs(names)
    if err != nil && expected {
        t.Errorf("`%v` expected to load", names)
    } else if err == nil && !expected {
        t.Errorf("`%v` expected throw error", names)
    }
}

func assertHome(t *testing.T, body []byte, expected interface{}) {
    raw := struct { Message string } {}
    if err := json.Unmarshal(body, &raw); err != nil {
        t.Errorf("Response body (%s) expected to be marshalled into struct (%#v)", string(body), raw)
        return
    }

    if raw.Message != expected.(struct { Message string }).Message {
        t.Errorf("Response message (%s) expected to be 'Welcome to the Hello World API!'", raw.Message)
    }
}

func assertGetLanguages(t *testing.T, body []byte, expected interface{}) {
    raw := &LanguagesResponse {}
    if err := json.Unmarshal(body, &raw); err != nil {
        t.Errorf("Response body (%s) expected to be marshalled into struct (%#v)", string(body), raw)
        return
    }

    diff := raw.RequestedAt.Sub(expected.(*LanguagesResponse).RequestedAt)
    if diff.Seconds() > 30  {
        t.Errorf("RequestedAt of Response (%v) and Expected (%v) expected to be within 30 seconds (%f)", raw.RequestedAt, expected.(*LanguagesResponse).RequestedAt, diff.Seconds())
    }
}

func assertGetLanguage(t *testing.T, body []byte, expected interface{}) {
    raw := &LanguageResponse {}
    if err := json.Unmarshal(body, &raw); err != nil {
        t.Errorf("Response body (%s) expected to be marshalled into struct (%#v)", string(body), raw)
        return
    }

    diff := raw.RequestedAt.Sub(expected.(*LanguageResponse).RequestedAt)
    if diff.Seconds() > 30  {
        t.Errorf("RequestedAt of Response (%v) and Expected (%v) expected to be within 30 seconds (%f)", raw.RequestedAt, expected.(*LanguageResponse).RequestedAt, diff.Seconds())
    }
}

// This is an additional assertion, testing common functionality to all routes.
func assertRoute(t *testing.T, fn handler, path string) []byte {
    req := httptest.NewRequest("GET", "http://localhost:8080" + path, nil)
    w := httptest.NewRecorder()
    fn(w, req)

    resp := w.Result()
    body, _ := io.ReadAll(resp.Body)

    if resp.StatusCode != 200 {
        t.Errorf("Status code (%d) expected to be 200", resp.StatusCode)
    }

    if resp.Header.Get("Content-Type") != "application/json" {
        t.Errorf("Content-Type (%s) expected to be 'application/json'", resp.Header.Get("Content-Type"))
    }

    return body
}

// --- STRUCTS ---

type handler func(w http.ResponseWriter, r *http.Request)

type authorizeTestCase struct {
    testName    string
    token       string
    expected    bool
}

type findLanguageTestCase struct {
    testName    string
    language    string
    expected    string
}

type findLanguagesTestCase struct {
    testName    string
    string      string
    expected    []*Language
}

type isLanguageTestCase struct {
    testName    string
    language    string
    path        string
    expected    bool
}

type loadConfigsTestCase struct {
    testName    string
    names       []string
    expected    bool
}

type routeTestCase struct {
    testName    string
    path        string
    handler     handler
    expected    interface{}
}
