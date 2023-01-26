package main

import (
    "fmt"
    "time"
    "regexp"
    "context"
    "strings"
    "net/http"
    "path/filepath"
    "encoding/json"

    "golang.org/x/oauth2"
    "github.com/spf13/viper"
    "github.com/gorilla/mux"
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
    RequestedAt     time.Time   `"json:requested_at"`
}

type LanguagesResponse struct {
    Languages       []*Language `"json:languages"`
    RequestedAt     time.Time   `"json:requested_at"`
}

var ctx context.Context

func home(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(struct { Message string } {
        Message: "Welcome to the Hello World API",
    })
}

func getLanguages(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    client := authorize(r.Header.Get("Authorization"))

    // Get the README object.
    readme, _, err := client.Repositories.GetReadme(
        ctx,
        viper.GetString("repository.user"),
        viper.GetString("repository.name"),
        nil,
    )

    if err != nil {
        w.WriteHeader(err.(*github.ErrorResponse).Response.StatusCode)
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    // Get the README contents.
    s, err := readme.GetContent()

    if err != nil {
        w.WriteHeader(err.(*github.ErrorResponse).Response.StatusCode)
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    languages := findLanguages(s)

    res := &LanguagesResponse{
        Languages: languages,
        RequestedAt: time.Now(),
    }

    json.NewEncoder(w).Encode(res)
}

func getLanguage(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    client := authorize(r.Header.Get("Authorization"))
    l := strings.TrimPrefix(r.URL.Path, "/api/language/")

    initial := strings.ToLower(l[0:1])

    if !regexp.MustCompile(`^[a-z]$`).MatchString(initial) {
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
        w.WriteHeader(err.(*github.ErrorResponse).Response.StatusCode)
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    rc := findLanguage(dir, l)

    if rc == nil {
        w.WriteHeader(http.StatusNotFound)
        return
    }

    name := rc.GetName()
    ext := filepath.Ext(name)
    n := strings.TrimSuffix(name, ext)

    language := &Language{
        Name: n,
        Extension: ext,
    }

    file, _, _, err := client.Repositories.GetContents(
        ctx,
        viper.GetString("repository.user"),
        viper.GetString("repository.name"),
        initial + "/" + name,
        nil,
    )

    if err != nil {
        w.WriteHeader(err.(*github.ErrorResponse).Response.StatusCode)
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    s, err := file.GetContent()

    if err != nil {
        w.WriteHeader(err.(*github.ErrorResponse).Response.StatusCode)
        json.NewEncoder(w).Encode(err.Error())
        return
    }

    code := &Code{
        Contents: s,
    }

    res := &LanguageResponse{
        Code: code,
        Language: language,
        RequestedAt: time.Now(),
    }

    json.NewEncoder(w).Encode(res)
}

func main() {
    router := mux.NewRouter()
    ctx = context.Background()

    // Load `env` configuration.
    viper.SetConfigName("env")
    viper.AddConfigPath("config")
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            fmt.Printf("Config file not found: %v\n", err)
        } else {
            fmt.Printf("Error: %v\n", err)
        }
        return
    }

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

func findLanguage(rcs []*github.RepositoryContent, l string) *github.RepositoryContent {
    for _, rc := range rcs {
        if isLanguage(rc, l) {
            return rc
        }
    }

    return nil
}

func findLanguages(s string) (languages []*Language) {
    re := regexp.MustCompile("\\* \\[(.+)\\]\\(.+\\)\n")

    // Find list of languages: "* [Language Name](lang.ext)"
    for _, m := range re.FindAllStringSubmatch(s, -1) {
        ext := filepath.Ext(m[0])

        languages = append(languages, &Language{
            Name: m[1],
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
