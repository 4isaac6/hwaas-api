package main

import (
    "fmt"
    "log"
    "time"
    "regexp"
    "context"
    "strings"
    "net/http"
    "encoding/json"

    "golang.org/x/oauth2"
    "github.com/spf13/viper"
    "github.com/gorilla/mux"
    "github.com/google/go-github/v49/github"
)

type Language struct {
    Name            string      `"json:name"`
    Extension       string      `"json:extension"`
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
    var languages   []*Language

    client := authorize(r.Header.Get("Authorization"))

    // Get the README object.
    readme, _, err := client.Repositories.GetReadme(
        ctx,
        viper.GetString("repository.user"),
        viper.GetString("repository.name"),
        nil,
    )

    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    // Get the README contents.
    s, _ := readme.GetContent()

    reExtension := regexp.MustCompile("(\\..+)")
    reLanguage  := regexp.MustCompile("\\* \\[(.+)\\]\\(.+\\)\n")

    // Find list of languages: "* [Language Name](lang.ext)"
    for _, m := range reLanguage.FindAllStringSubmatch(s, -1) {
        // Not all languages have file extensions; search separately.
        ext := reExtension.FindString(m[0])

        languages = append(languages, &Language{
            Name: m[1],
            Extension: ext,
        })
    }

    res := &LanguagesResponse{
        Languages: languages,
        RequestedAt: time.Now(),
    }

    w.Header().Set("Content-Type", "application/json")
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
    		// Config file not found
	        fmt.Printf("Error: %v\n", err)
    	} else {
    		// Config file was found but another error was produced
            fmt.Printf("Error: %v\n", err)
    	}
        return
    }

    router.HandleFunc("/api", home).Methods("GET")
    router.HandleFunc("/api/languages", getLanguages).Methods("GET")
    // router.HandleFunc("/api/languages/{language}", getLanguage).Methods("GET")

	log.Fatal(http.ListenAndServe(
        fmt.Sprintf(":%d", viper.GetInt("server.port")),
        router,
    ))
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
