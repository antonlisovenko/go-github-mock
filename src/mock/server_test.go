package mock

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-github/v37/github"
)

func TestNewMockedHTTPClient(t *testing.T) {
	mockedHTTPClient := NewMockedHTTPClient(
		WithRequestMatch(
			GetUsersByUsername,
			[][]byte{
				MustMarshal(github.User{
					Name: github.String("foobar"),
				}),
			},
		),
		WithRequestMatch(
			GetUsersOrgsByUsername,
			[][]byte{
				MustMarshal([]github.Organization{
					{
						Name: github.String("foobar123thisorgwasmocked"),
					},
				}),
			},
		),
		WithRequestMatchHandler(
			GetOrgsProjectsByOrg,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write(MustMarshal([]github.Project{
					{
						Name: github.String("mocked-proj-1"),
					},
					{
						Name: github.String("mocked-proj-2"),
					},
				}))
			}),
		),
	)
	c := github.NewClient(mockedHTTPClient)

	ctx := context.Background()

	user, _, userErr := c.Users.Get(ctx, "someUser")

	if user == nil || user.Name == nil || *user.Name != "foobar" {
		t.Fatalf("user name is %s, want foobar", user)
	}

	if userErr != nil {
		t.Errorf("user err is %s, want nil", userErr.Error())
	}

	orgs, _, orgsErr := c.Organizations.List(
		ctx,
		*(user.Name),
		nil,
	)

	if len(orgs) != 1 {
		t.Errorf("orgs len is %d want 1", len(orgs))
	}

	if orgsErr != nil {
		t.Errorf("orgs err is %s, want nil", orgsErr.Error())
	}

	if *(orgs[0].Name) != "foobar123thisorgwasmocked" {
		t.Errorf("orgs[0].Name is %s, want %s", *orgs[0].Name, "foobar123thisorgdoesnotexist")
	}

	projs, _, projsErr := c.Organizations.ListProjects(
		ctx,
		*orgs[0].Name,
		&github.ProjectListOptions{},
	)

	if projsErr != nil {
		t.Errorf("projs err is %s, want nil", projsErr.Error())
	}

	if len(projs) != 2 {
		t.Errorf("projs len is %d want 2", len(projs))
	}
}

func TestMockErrors(t *testing.T) {
	mockedHTTPClient := NewMockedHTTPClient(
		WithRequestMatchHandler(
			GetUsersByUsername,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteError(
					w,
					http.StatusInternalServerError,
					"github went belly up or something",
				)
			}),
		),
	)
	c := github.NewClient(mockedHTTPClient)

	ctx := context.Background()

	user, _, userErr := c.Users.Get(ctx, "someUser")

	if userV := reflect.ValueOf(user); !userV.IsZero() {
		t.Errorf("user is %s, want nil", user)
	}

	if userErr == nil {
		t.Fatal("user err is nil, want *github.ErrorResponse")
	}

	ghErr, ok := userErr.(*github.ErrorResponse)

	if !ok {
		t.Fatal("couldn't cast userErr to *github.ErrorResponse")
	}

	if ghErr.Message != "github went belly up or something" {
		t.Errorf("user err is %s, want 'github went belly up or something'", userErr.Error())
	}
}

func TestMocksNotConfiguredError(t *testing.T) {
	mockedHTTPClient := NewMockedHTTPClient(
		WithRequestMatch(
			GetUsersByUsername,
			[][]byte{
				MustMarshal(github.User{
					Name: github.String("foobar"),
				}),
			},
		),
	)
	c := github.NewClient(mockedHTTPClient)

	ctx := context.Background()

	user, _, userErr := c.Users.Get(ctx, "someUser")

	if user == nil || user.Name == nil || *user.Name != "foobar" {
		t.Fatalf("user name is %s, want foobar", user)
	}

	if userErr != nil {
		t.Errorf("user err is %s, want nil", userErr.Error())
	}

	orgs, _, orgsErr := c.Organizations.List(
		ctx,
		"someuser",
		&github.ListOptions{},
	)

	if len(orgs) > 0 {
		t.Errorf("orgs len is %d, want 0", len(orgs))
	}

	if r, ok := orgsErr.(*github.ErrorResponse); ok {
		if !strings.Contains(r.Message, "mock response not found for") {
			t.Errorf("error message should contain 'mock response not found for'")
		}
	}
}

func TestMocksPaginationAllPages(t *testing.T) {
	mockedHTTPClient := NewMockedHTTPClient(
		WithRequestMatchPages(
			GetOrgsReposByOrg,
			[][]byte{
				MustMarshal([]github.Repository{
					{
						Name: github.String("repo-A-on-first-page"),
					},
					{
						Name: github.String("repo-B-on-first-page"),
					},
				}),
				MustMarshal([]github.Repository{
					{
						Name: github.String("repo-C-on-second-page"),
					},
					{
						Name: github.String("repo-D-on-second-page"),
					},
				}),
			},
		),
	)

	c := github.NewClient(mockedHTTPClient)

	ctx := context.Background()

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{
			// in fact, the perPage option is ignored
			// but this would be present in production code
			PerPage: 2,
		},
	}

	var allRepos []*github.Repository

	for {
		repos, resp, listErr := c.Repositories.ListByOrg(ctx, "foobar", opt)

		if listErr != nil {
			t.Errorf("error listing repositories: %s", listErr.Error())
		}

		// matches mock definition
		if len(repos) != 2 {
			t.Errorf("len(repos) is %d, want 2", len(repos))
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	if len(allRepos) != 4 {
		t.Errorf("len(allRepos) is %d, want 4", len(allRepos))
	}
}

func TestMocksEmptyResult(t *testing.T) {
	mockedHTTPClient := NewMockedHTTPClient(
		WithRequestMatch(
			GetReposReleasesByOwnerByRepo,
			[][]byte{},
		),
		WithRequestMatchPages(
			GetReposCommitsByOwnerByRepo,
			[][]byte{},
		),
	)
	c := github.NewClient(mockedHTTPClient)

	ctx := context.Background()

	releases, _, releaseErr := c.Repositories.ListReleases(ctx, "thecompany", "therepo", &github.ListOptions{})

	if releases != nil {
		t.Fatalf("releases is %s, want nil", releases)
	}

	if releaseErr != nil {
		t.Errorf("releases err is %s, want nil", releaseErr.Error())
	}

	commits, _, orgsErr := c.Repositories.ListCommits(
		ctx,
		"thecompany",
		"repo",
		&github.CommitsListOptions{},
	)

	if len(commits) != 0 {
		t.Errorf("commits len is %d want 0", len(commits))
	}

	if orgsErr != nil {
		t.Errorf("orgs err is %s, want nil", orgsErr.Error())
	}
}
