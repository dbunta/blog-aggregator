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
	res, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}
	fmt.Printf("%v", res)
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
