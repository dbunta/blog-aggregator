package main

import (
	"fmt"
	"os"

	config "github.com/dbunta/blog-aggregator/internal/config"
)

func main() {
	c, err := config.Read()
	if err != nil {
		fmt.Println(fmt.Errorf("Error getting config: %w", err))
		os.Exit(1)
	}
	var st state
	st.config = &c

	var cmds commands
	cmds.handlers = make(map[string]func(*state, command) error)
	cmds.register("login", handlerLogin)

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

	//c.SetUser("lane")
	//c, err = config.Read()
	//fmt.Print(c)
}

type state struct {
	config *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
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
	err := s.config.SetUser(cmd.args[0])
	if len(cmd.args) == 0 {
		return fmt.Errorf("login error: %w", err)
	}
	fmt.Printf("Current user has been set to %v\n", cmd.args[0])
	return nil
}
