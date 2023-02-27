package main

import (
    "fmt"
    "time"
    "bytes"
    "regexp"
    "context"
    "strings"
    "net/url"
    "net/http"
    "encoding/gob"
    "encoding/json"
    "path/filepath"

    "golang.org/x/oauth2"
    "github.com/spf13/viper"
    "github.com/gorilla/mux"
	"github.com/allegro/bigcache/v3"
    "github.com/google/go-github/v49/github"
)

type Code struct {
    Contents        string      `"json:contents"`
}

type Language struct {
    Name            string      `"json:name"`
    Extension       string      `"json:extension"`
}

type LanguageResponse struct {
    Code            *Code       `"json:code"`
    Language        *Language   `"json:language"`
    CachedAt        time.Time   `"json:cached_at"`
    RequestedAt     time.Time   `"json:requested_at"`
}

type LanguagesResponse struct {
    Languages       []*Language `"json:languages"`
    CachedAt        time.Time   `"json:cached_at"`
    RequestedAt     time.Time   `"json:requested_at"`
}

// Stolen from: https://github.com/google/go-github/blob/838d2238a6da019b49b571e8d8ebc5a6b12f8844/github/github.go#L863
type ErrorResponse struct {
    Request         *http.Request
    StatusCode      int         `json:"status_code"`
    Message         string      `json:"message"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %v",
		r.Request.Method, r.Request.URL,
		r.StatusCode, r.Message)
}

var ctx context.Context
var cache *bigcache.BigCache

func home(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(struct { Message string } {
        Message: "Welcome to the Hello World API!",
    })
}

func getLanguages(w http.ResponseWriter, r *http.Request) {
    var res LanguagesResponse

    w.Header().Set("Content-Type", "application/json")

    if err := cacheGet("languages", &res); err == nil {
        res.RequestedAt = time.Now()
        json.NewEncoder(w).Encode(res)
        return
    }

    client := authorize(r.Header.Get("Authorization"))

    // Get the README object.
    readme, _, err := client.Repositories.GetReadme(
        ctx,
        viper.GetString("repository.user"),
        viper.GetString("repository.name"),
        nil,
    )

    if err != nil {
        if err, ok := err.(*github.ErrorResponse); ok {
            w.WriteHeader(err.Response.StatusCode)
        } else {
            w.WriteHeader(http.StatusInternalServerError)
        }
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    // Get the README contents.
    s, err := readme.GetContent()

    if err != nil {
        if err, ok := err.(*github.ErrorResponse); ok {
            w.WriteHeader(err.Response.StatusCode)
        } else {
            w.WriteHeader(http.StatusInternalServerError)
        }
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    languages := findLanguages(s)

    res = LanguagesResponse{
        Languages: languages,
        CachedAt: time.Now(),
        RequestedAt: time.Now(),
    }

    cacheSet("languages", res)

    json.NewEncoder(w).Encode(res)
}

func getLanguage(w http.ResponseWriter, r *http.Request) {
    var res LanguageResponse

    w.Header().Set("Content-Type", "application/json")

    l := strings.TrimPrefix(r.URL.Path, "/api/language/")

    if err := cacheGet("language-" + l, &res); err == nil {
        res.RequestedAt = time.Now()
        json.NewEncoder(w).Encode(res)
        return
    }

    initial := strings.ToLower(l[0:1])
    client := authorize(r.Header.Get("Authorization"))

    if !regexp.MustCompile("^[a-z]$").MatchString(initial) {
        initial = "#"
    }

    _, dir, _, err := client.Repositories.GetContents(
        ctx,
        viper.GetString("repository.user"),
        viper.GetString("repository.name"),
        initial,
        nil,
    )

    if err != nil {
        if err, ok := err.(*github.ErrorResponse); ok {
            w.WriteHeader(err.Response.StatusCode)
        } else {
            w.WriteHeader(http.StatusInternalServerError)
        }
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    language, err := findLanguage(dir, l)

    if err != nil {
        if err, ok := err.(*ErrorResponse); ok {
            err.Request = r
            w.WriteHeader(err.StatusCode)
        } else {
            w.WriteHeader(http.StatusInternalServerError)
        }
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    file, _, _, err := client.Repositories.GetContents(
        ctx,
        viper.GetString("repository.user"),
        viper.GetString("repository.name"),
        initial + "/" + language.Name + language.Extension,
        nil,
    )

    if err != nil {
        if err, ok := err.(*github.ErrorResponse); ok {
            w.WriteHeader(err.Response.StatusCode)
        } else {
            w.WriteHeader(http.StatusInternalServerError)
        }
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    s, err := file.GetContent()

    if err != nil {
        if err, ok := err.(*github.ErrorResponse); ok {
            w.WriteHeader(err.Response.StatusCode)
        } else {
            w.WriteHeader(http.StatusInternalServerError)
        }
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    code := &Code{
        Contents: s,
    }

    res = LanguageResponse{
        Code: code,
        Language: language,
        CachedAt: time.Now(),
        RequestedAt: time.Now(),
    }

    cacheSet("language-" + l, res)

    json.NewEncoder(w).Encode(res)
}

func main() {
    router := mux.NewRouter()
    ctx = context.Background()
    cache, _ = bigcache.New(ctx, bigcache.DefaultConfig(24 * time.Hour))

    loadConfigs([]string {
        "env",
    })

    router.HandleFunc("/api", home).Methods(http.MethodGet)
    router.HandleFunc("/api/languages", getLanguages).Methods(http.MethodGet)
    router.HandleFunc("/api/language/{language}", getLanguage).Methods(http.MethodGet)

    http.ListenAndServe(
        ":" + viper.GetString("server.port"),
        router,
    )
}

// --- HELPERS ---

func authorize(s string) *github.Client {
    if s == "" {
        return github.NewClient(nil)
    }

    t := strings.Replace(s, "Bearer", "", 1)
    ts := oauth2.StaticTokenSource(
        &oauth2.Token{AccessToken: t},
    )
    tc := oauth2.NewClient(ctx, ts)

    return github.NewClient(tc)
}

func cacheGet(key string, res interface{}) error {
    entry, err := cache.Get(key)

    if err != nil {
        return err
    }

    buffer := bytes.NewBuffer(entry)
    dec := gob.NewDecoder(buffer)
    err = dec.Decode(res)

    if err != nil {
        // If the cache entry throws an error when decoding, delete it and fetch it again.
        cache.Delete(key)
        return err
    }

    return nil
}

func cacheSet(key string, value interface{}) error {
    buffer := bytes.NewBuffer([]byte{})
    enc := gob.NewEncoder(buffer)
    err := enc.Encode(value)

    if err != nil {
        return err
    }

    return cache.Set(key, buffer.Bytes())
}

func findLanguage(rcs []*github.RepositoryContent, l string) (*Language, error) {
    for _, rc := range rcs {
        if isLanguage(rc, l) {
            name := rc.GetName()
            ext := filepath.Ext(name)
            n := strings.TrimSuffix(name, ext)

            return &Language{
                Name: n,
                Extension: ext,
            }, nil
        }
    }

    return nil, &ErrorResponse{
        Request: nil,
		StatusCode: http.StatusNotFound,
		Message: "Not Found",
	}
}

func findLanguages(s string) (languages []*Language) {
    // Find language name and extension in link.
    re := regexp.MustCompile("\\* \\[.+\\]\\((?:[a-z]|%23)/(.+)\\)\n")

    // Find list of languages: "* [Language Name](lang.ext)"
    for _, m := range re.FindAllStringSubmatch(s, -1) {
        filename, err := url.PathUnescape(m[1])

        if err != nil {
            continue
        }

        ext := filepath.Ext(filename)
        name := strings.TrimSuffix(filename, ext)

        languages = append(languages, &Language{
            Name: name,
            Extension: ext,
        })
    }

    return
}

func isLanguage(rc *github.RepositoryContent, l string) bool {
    name := rc.GetName()
    ext := filepath.Ext(name)
    n := strings.TrimSuffix(name, ext)

    return strings.ToLower(l) == strings.ToLower(n)
}

func loadConfigs(configs []string) (err error) {
    viper.AddConfigPath("config")

    for _, s := range configs {
        viper.SetConfigName(s)

        if err = viper.MergeInConfig(); err != nil {
            return
        }
    }

    return
}
