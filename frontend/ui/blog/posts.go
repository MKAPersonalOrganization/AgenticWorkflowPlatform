// Package blog hosts the public-facing technical blog. Posts are authored
// as templ components (no markdown renderer — see CLAUDE memory
// feedback_content_authoring.md) and registered in this file.
//
// Adding a new post:
//  1. Create a new templ file `post_<slug>.templ` exposing a templ component.
//  2. Append a Post{} entry to allPosts below.
//  3. `templ generate && go build ./...` — that's it, no migration, no deps.
package blog

import (
	"time"

	"github.com/a-h/templ"
)

// Post is a single blog entry. The Body field is a templ component so we can
// reuse the docsSection / templui helpers we already have for the /docs page.
type Post struct {
	Slug    string
	Title   string
	Summary string
	Author  string
	Date    time.Time
	Tags    []string
	Body    templ.Component
}

// mustDate panics if the layout-2006-01-02 string fails to parse — acceptable
// at package-init time since the input is a compile-time constant.
func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic("blog: invalid date " + s + ": " + err.Error())
	}
	return t
}

// allPosts is the registry of published blog posts, newest first.
// Keep this list in reverse-chronological order.
var allPosts = []Post{
	{
		Slug:    "agent-as-tool-pattern",
		Title:   "Agent as Tool: the function-call multi-agent pattern",
		Summary: "Two agents, two directions. When the parent must keep control of the final answer, don't transfer — call.",
		Author:  "Compass.AI Engineering",
		Date:    mustDate("2026-04-17"),
		Tags:    []string{"multi-agent", "ADK", "patterns"},
		Body:    postAgentAsToolBody(),
	},
}

// AllPosts returns the published posts in reverse-chronological order.
func AllPosts() []Post {
	out := make([]Post, len(allPosts))
	copy(out, allPosts)
	return out
}

// FindPost returns a post by slug, or nil if no such post exists.
func FindPost(slug string) *Post {
	for i := range allPosts {
		if allPosts[i].Slug == slug {
			return &allPosts[i]
		}
	}
	return nil
}
