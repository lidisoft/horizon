package vcs

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Providers.
const (
	GitHub  = "github"
	GitLab  = "gitlab"
	Forgejo = "forgejo"
)

// Providers lists the supported forges.
var Providers = []string{GitHub, GitLab, Forgejo}

// Pull request and branch states.
const (
	StateOpen    = "open"
	StateMerged  = "merged"
	StateClosed  = "closed"
	StateDeleted = "deleted"
)

// delivery is an inbound forge webhook, made provider-neutral.
type delivery struct {
	// Repository is "owner/name" (GitLab: the path with namespace).
	Repository string
	RepoURL    string
	// ProjectRef is GitLab's numeric project id, for its API.
	ProjectRef string
	// Branch is a branch created, pushed to or deleted ("" for none).
	Branch        string
	BranchDeleted bool
	Commits       []commit
	PullRequest   *pullRequest
	// Ping is the forge's test delivery: nothing to link.
	Ping bool
}

type commit struct {
	SHA, Message, URL, Author string
}

type pullRequest struct {
	Number      int
	Title, Body string
	URL, Branch string
	State       string // open, merged, closed
	Author      string
}

var errSignature = errors.New("vcs: bad or missing signature")

// verify checks the delivery came from the forge holding secret: GitHub
// and Forgejo sign the body with HMAC-SHA256, GitLab sends the secret as a
// token.
func verify(provider, secret string, h http.Header, body []byte) error {
	var got []byte
	var want []byte
	switch provider {
	case GitLab:
		got, want = []byte(h.Get("X-Gitlab-Token")), []byte(secret)
	case GitHub, Forgejo:
		sig := h.Get("X-Hub-Signature-256")
		if provider == Forgejo && sig == "" {
			sig = "sha256=" + h.Get("X-Forgejo-Signature")
		}
		hexSig, ok := strings.CutPrefix(sig, "sha256=")
		if !ok {
			return errSignature
		}
		var err error
		if got, err = hex.DecodeString(hexSig); err != nil {
			return errSignature
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		want = mac.Sum(nil)
	}
	if len(got) == 0 || subtle.ConstantTimeCompare(got, want) != 1 {
		return errSignature
	}
	return nil
}

// eventName is the delivery's event, as the forge names it.
func eventName(provider string, h http.Header) string {
	switch provider {
	case GitLab:
		return h.Get("X-Gitlab-Event")
	case Forgejo:
		if e := h.Get("X-Forgejo-Event"); e != "" {
			return e
		}
		return h.Get("X-Gitea-Event")
	}
	return h.Get("X-GitHub-Event")
}

// zeroSHA marks a created or deleted ref in push payloads.
const zeroSHA = "0000000000000000000000000000000000000000"

// GitHub and Forgejo share a payload shape.
type hubRepo struct {
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

type hubUser struct {
	Login string `json:"login"`
}

type hubPayload struct {
	Ref        string  `json:"ref"`
	RefType    string  `json:"ref_type"`
	Before     string  `json:"before"`
	After      string  `json:"after"`
	Deleted    bool    `json:"deleted"`
	Repository hubRepo `json:"repository"`
	Commits    []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		URL     string `json:"url"`
		Author  struct {
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"author"`
	} `json:"commits"`
	Action      string `json:"action"`
	PullRequest *struct {
		Number  int     `json:"number"`
		Title   string  `json:"title"`
		Body    string  `json:"body"`
		HTMLURL string  `json:"html_url"`
		State   string  `json:"state"`
		Merged  bool    `json:"merged"`
		User    hubUser `json:"user"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
}

type gitlabPayload struct {
	ObjectKind string `json:"object_kind"`
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	UserName   string `json:"user_name"`
	User       struct {
		Username string `json:"username"`
	} `json:"user"`
	Project struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		URL     string `json:"url"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
	ObjectAttributes *struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		URL          string `json:"url"`
		State        string `json:"state"`
		SourceBranch string `json:"source_branch"`
	} `json:"object_attributes"`
}

// errIgnored is a delivery Horizon has no use for.
var errIgnored = errors.New("vcs: event ignored")

// parse reads a delivery of the given event.
func parse(provider, event string, body []byte) (delivery, error) {
	if provider == GitLab {
		return parseGitLab(event, body)
	}
	var d delivery
	if event == "ping" {
		d.Ping = true
		return d, nil
	}
	var p hubPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return d, err
	}
	d.Repository, d.RepoURL = p.Repository.FullName, p.Repository.HTMLURL
	switch event {
	case "push":
		d.Branch, _ = strings.CutPrefix(p.Ref, "refs/heads/")
		if d.Branch == p.Ref {
			return d, errIgnored // a tag
		}
		d.BranchDeleted = p.Deleted || p.After == zeroSHA
		for _, c := range p.Commits {
			author := c.Author.Username
			if author == "" {
				author = c.Author.Name
			}
			d.Commits = append(d.Commits, commit{SHA: c.ID, Message: c.Message, URL: c.URL, Author: author})
		}
	case "create", "delete":
		if p.RefType != "branch" {
			return d, errIgnored
		}
		d.Branch = strings.TrimPrefix(p.Ref, "refs/heads/")
		d.BranchDeleted = event == "delete"
	case "pull_request":
		pr := p.PullRequest
		if pr == nil {
			return d, errIgnored
		}
		state := StateOpen
		switch {
		case pr.Merged:
			state = StateMerged
		case pr.State == "closed":
			state = StateClosed
		}
		d.PullRequest = &pullRequest{Number: pr.Number, Title: pr.Title, Body: pr.Body, URL: pr.HTMLURL, Branch: pr.Head.Ref,
			State: state, Author: pr.User.Login}
	default:
		return d, errIgnored
	}
	return d, nil
}

func parseGitLab(event string, body []byte) (delivery, error) {
	var d delivery
	var p gitlabPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return d, err
	}
	d.Repository, d.RepoURL = p.Project.PathWithNamespace, p.Project.WebURL
	if p.Project.ID != 0 {
		d.ProjectRef = strconv.Itoa(p.Project.ID)
	}
	switch {
	case event == "Push Hook" || p.ObjectKind == "push":
		d.Branch, _ = strings.CutPrefix(p.Ref, "refs/heads/")
		if d.Branch == p.Ref {
			return d, errIgnored
		}
		d.BranchDeleted = p.After == zeroSHA
		for _, c := range p.Commits {
			d.Commits = append(d.Commits, commit{SHA: c.ID, Message: c.Message, URL: c.URL, Author: c.Author.Name})
		}
	case event == "Merge Request Hook" || p.ObjectKind == "merge_request":
		mr := p.ObjectAttributes
		if mr == nil {
			return d, errIgnored
		}
		state := StateOpen
		switch mr.State {
		case "merged":
			state = StateMerged
		case "closed":
			state = StateClosed
		}
		d.PullRequest = &pullRequest{Number: mr.IID, Title: mr.Title, Body: mr.Description, URL: mr.URL, Branch: mr.SourceBranch,
			State: state, Author: p.User.Username}
	case event == "System Hook":
		return d, errIgnored
	default:
		return d, errIgnored
	}
	return d, nil
}
