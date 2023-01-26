# hwaas-api
This API solves every developer's most pressing challenge: "How do I start a file in this language again?"

### Inspiration
This project was principally inspired by [Fuck Off as a Service](https://github.com/tomdionysus/foaas).

While still a novel API, the data is from the [Hello World in every computer language](https://github.com/leachim6/hello-world) repository, pulled using the GitHub API.

### Usage
| Route             | Method | Result |
|-------------------|--------|--------|
| `/api`            |  GET   | A welcome message to the API |
| `/api/languages`  |  GET   | Displays all available languages |
| `/api/{language}` |  GET   | Returns the code required for a "Hello World!" program in the given language, if it exists in the repository |
