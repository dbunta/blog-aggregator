package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	config "github.com/dbunta/blog-aggregator/internal/config"
	"github.com/dbunta/blog-aggregator/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func main() {
	c, err := config.Read()
	if err != nil {
		fmt.Println(fmt.Errorf("Error getting config: %w", err))
		os.Exit(1)
	}

	db, err := sql.Open("postgres", c.DbUrl)
	dbQueries := database.New(db)

	var st state
	st.config = &c
	st.db = dbQueries

	var cmds commands
	cmds.handlers = make(map[string]func(*state, command) error)
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerGetUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerGetFeedFollows))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", handlerBrowse)

	args := os.Args
	if len(args) < 2 {
		fmt.Println(fmt.Errorf("missing arguments"))
		os.Exit(1)
	}

	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}
	err = cmds.run(&st, cmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, c command) error {
		user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
		if err != nil {
			return fmt.Errorf("get feeds error: %w", err)
		}
		return handler(s, c, user)
	}
}

type state struct {
	config *config.Config
	db     *database.Queries
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type feed struct {
	name      string `json:"name"`
	url       string `json:"url"`
	user_name string `json:"user_name"`
}

func (c *commands) run(s *state, cmd command) error {
	fn, ok := c.handlers[cmd.name]
	if !ok {
		return fmt.Errorf("command not found")
	}
	return fn(s, cmd)
}

func (c *commands) register(name string, f func(*state, command) error) error {
	_, ok := c.handlers[name]
	if ok {
		return fmt.Errorf("command already registered")
	}
	c.handlers[name] = f
	return nil
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("login error: no username provided in args")
	}

	user, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("login error: user does not exist")
	}

	err = s.config.SetUser(user.Name)
	if len(cmd.args) == 0 {
		return fmt.Errorf("login error: %w", err)
	}
	fmt.Printf("Current user has been set to %v\n", cmd.args[0])
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("register error: no name provided in args")
	}
	params := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
	}

	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err == nil {
		return fmt.Errorf("register error: user with that name already exists: %w", err)
	}

	newUser, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("register error: error creating user: %w", err)
	}

	err = s.config.SetUser(newUser.Name)
	if len(cmd.args) == 0 {
		return fmt.Errorf("login error: %w", err)
	}
	fmt.Printf("new user %v was created\n", newUser.Name)
	fmt.Printf("new user info: %v\n", newUser)
	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.TruncateUsers(context.Background())
	if err != nil {
		return fmt.Errorf("reset error: error truncating users table: %w", err)
	}
	fmt.Printf("users table successfully reset\n")
	return nil
}

func handlerGetUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("get users error: error getting users: %w", err)
	}

	for i := 0; i < len(users); i++ {
		fmt.Printf("* %v", users[i].Name)
		if s.config.CurrentUserName == users[i].Name {
			fmt.Printf(" (current)")
		}
		fmt.Printf("\n")
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("agg error: no time between reqs was provided")
	}

	timeBetweenReqs, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("agg error: %w", err)
	}

	ticker := time.NewTicker(timeBetweenReqs)
	for ; ; <-ticker.C {
		fmt.Printf("Collecting feeds every %s\n", timeBetweenReqs)
		err = scrapeFeeds(s, cmd)
		if err != nil {
			return fmt.Errorf("agg error: %w", err)
		}
	}

	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch feed error 1: %w", err)
	}
	req.Header.Set("User-Agent", "gator")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch feed error 2: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch feed error 3: %w", err)
	}

	var rssFeed RSSFeed
	err = xml.Unmarshal(body, &rssFeed)
	if err != nil {
		return nil, fmt.Errorf("fetch feed error 4: %w", err)
	}

	rssFeed.Channel.Description = html.UnescapeString(rssFeed.Channel.Description)
	rssFeed.Channel.Title = html.UnescapeString(rssFeed.Channel.Title)
	for i := 0; i < len(rssFeed.Channel.Item); i++ {
		rssFeed.Channel.Item[i].Description = html.UnescapeString(rssFeed.Channel.Item[i].Description)
		rssFeed.Channel.Item[i].Title = html.UnescapeString(rssFeed.Channel.Item[i].Title)
	}

	return &rssFeed, nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("add feed error: name and url must be provided")
	}

	params := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	}

	feed, err := s.db.CreateFeed(context.Background(), params)
	if err != nil {
		return fmt.Errorf("add feed error: %v", err)
	}

	followParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	}
	_, err = s.db.CreateFeedFollow(context.Background(), followParams)
	if err != nil {
		return fmt.Errorf("follows error: %w", err)
	}

	fmt.Printf("%v", feed)

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("get feeds error: %v", err)
	}
	for _, v := range feeds {
		fmt.Printf("%v\n", v)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("follows error: url not provided")
	}

	feed, err := s.db.GetFeed(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("follows error: %w", err)
	}

	params := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	}
	ff, err := s.db.CreateFeedFollow(context.Background(), params)
	if err != nil {
		return fmt.Errorf("follows error: %w", err)
	}
	fmt.Printf("%v is now following %v", ff.UserName, ff.FeedName)
	return nil
}

func handlerGetFeedFollows(s *state, cmd command, user database.User) error {
	feeds, err := s.db.GetFeedFollows(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("get feeds error: %w", err)
	}

	for _, f := range feeds {
		fmt.Printf("%v\n", f.FeedName)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("unfollow error: feed url required")
	}
	params := database.DeleteFeedFollowsParams{
		Name: user.Name,
		Url:  cmd.args[0],
	}
	err := s.db.DeleteFeedFollows(context.Background(), params)
	if err != nil {
		return fmt.Errorf("unfollow error: %w", err)
	}
	fmt.Printf("%v unfollowed feed: %v\n", user.Name, cmd.args[0])
	return nil
}

func scrapeFeeds(s *state, cmd command) error {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("scrape feeds error: %w", err)
	}

	params := database.MarkFeedFetchedParams{
		LastFetchedAt: sql.NullTime{Time: time.Now(), Valid: true},
		ID:            feed.ID,
	}
	err = s.db.MarkFeedFetched(context.Background(), params)
	if err != nil {
		return fmt.Errorf("scrape feeds error: %w", err)
	}

	rss, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		return fmt.Errorf("scrape feeds error: %w", err)
	}

	fmt.Printf("%v\n", rss.Channel.Title)
	for _, val := range rss.Channel.Item {
		pubDate, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", val.PubDate)
		if err != nil {
			return fmt.Errorf("scrape feeds error: %w", err)
		}
		params := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       val.Title,
			Url:         val.Link,
			Description: val.Description,
			PublishedAt: pubDate,
			FeedID:      feed.ID,
		}
		_, err = s.db.CreatePost(context.Background(), params)
		if err != nil && !strings.Contains(err.Error(), "duplicate key value violates unique constraint \"posts_url_key\"") {
			return fmt.Errorf("scrape feeds error: %w", err)
		}
		fmt.Printf("%v\n", val.Title)
	}
	return nil
}

func handlerBrowse(s *state, cmd command) error {
	var limit int64 = 2
	if len(cmd.args) > 0 {
		parsedLimit, err := strconv.ParseInt(cmd.args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("browse error: %w", err)
		}
		limit = parsedLimit
	}
	params := database.GetPostsForUserParams{
		Name:  s.config.CurrentUserName,
		Limit: int32(limit),
	}
	posts, err := s.db.GetPostsForUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("browse error: %w", err)
	}

	for _, post := range posts {
		fmt.Printf("-----------------------------------------------------------\n")
		fmt.Printf("%v\n", post)
		fmt.Printf("-----------------------------------------------------------\n")
	}
	return nil
}
