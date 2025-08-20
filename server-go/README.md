## Go API for React Quiz

This API accepts login submissions and appends them to a JSON file in a GitHub repository using the GitHub Contents API.

### Environment

Create a `.env` file with:

```
GITHUB_TOKEN=ghp_xxx
GITHUB_REPO=owner/repo
GITHUB_FILE_PATH=data/submissions.json
GITHUB_BRANCH=main
PORT=8080
CORS_ALLOW_ORIGIN=http://localhost:5173
```

The token needs `repo` scope for private repos or `public_repo` for public repos.

### Run

```
go run .
```


