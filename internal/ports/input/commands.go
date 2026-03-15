package input

type Command interface {
	IsCommand()
}

type RefreshCommand struct{}

func (RefreshCommand) IsCommand() {}

type QuitCommand struct{}

func (QuitCommand) IsCommand() {}
