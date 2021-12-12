package main

// An example Bubble Tea server. This will put an ssh session into alt screen
// and continually print up to date terminal information.

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"time"

	clip "github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/charm/kv"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/gliderlabs/ssh"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

const host = "localhost"
const port = 22

var globalDB *kv.KV = nil

type tickMsg time.Time


func main() {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
		wish.WithHostKeyPath(".ssh/term_info_ed25519"),
		wish.WithMiddleware(
			bm.Middleware(teaHandler),
			lm.Middleware(),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
	// Open a database (or create one if it doesn’t exist)
	db, err := kv.OpenWithDefaults("code-cube-pastes")
	if err != nil {
		log.Fatal(err)
	}
	globalDB = db
	// Other stuff
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("Starting SSH server on %s:%d", host, port)
	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	<-done
	log.Println("Stopping SSH server")
	if err := s.Close(); err != nil {
		log.Fatalln(err)
	}
}

// You can wire any Bubble Tea model up to the middleware with a function that
// handles the incoming ssh.Session. Here we just grab the terminal info and
// pass it to the new model. You can also return tea.ProgramOptions (such as
// teaw.WithAltScreen) on a session by session basis
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, active := s.Pty()
	if !active {
		fmt.Println("no active terminal, skipping")
		return nil, nil
	}
	ti := textinput.NewModel()
	ti.Placeholder = "Hello, World!"
	ti.Focus()
	ti.CharLimit = 100000 // 100,000 character limit
	ti.Width = 20

	m := model{
		term:   pty.Term,
		width:  pty.Window.Width,
		height: pty.Window.Height,
		textInput: ti,
		progress: progress.NewModel(progress.WithDefaultGradient()),
	}
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	term   string
	width  int
	height int
	textInput textinput.Model
	savedId string
	state string
	progress progress.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tickMsg:
		cmd = m.progress.IncrPercent(0.25)
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "x", "X":
			if (m.state == "") {
				m.state = "newPaste"
				return m, nil
			}
		case "r", "R":
			if (m.state == ""){
				 m.state = "getPaste"
				 return m, nil
			}
		case "a", "A":
			if (m.state == "") {
				m.state = "flavor"
				return m,nil
			}
		case "b", "B":
			if (m.state == "flavor") {
				m.state = ""
				return m,nil
			}
		case "enter":
			if (m.state == "newPaste" && m.textInput.Value() != "") {
				// Generate ID
				id, err := gonanoid.Generate("qwertyuiopasdfghjlzxcvbnm1234567890", 8)
				if err != nil {
					log.Fatal(err)
				}
				m.state = "loading"
				// Save some data
				if err := globalDB.Set([]byte(id), []byte(m.textInput.Value())); err != nil {
					log.Fatal(err)
				}
				// Update UI
				m.savedId = id
				m.state = "pasted"
			} else if (m.state == "getPaste" && m.textInput.Value() != "") {
				// Fetch updates and easily define your own syncing strategy
				if err := globalDB.Sync(); err != nil {
					log.Fatal(err)
				}
				m.state = "loading"
				result, err := globalDB.Get([]byte(m.textInput.Value()))
				if (err != nil) {
					if (err.Error() == "Key not found") {
						m.state = "keyNotFound"
					} else {
						log.Fatal(err)
					}
				} else {
					clip.WriteAll(string(result))
					m.state = "copied"
				}
			}
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, tea.Batch(cmd,tickCmd())
}

func (m model) View() string {
	var title = lipgloss.NewStyle().
    Bold(true).Foreground(lipgloss.Color("#874BFC"))

	var bold = lipgloss.NewStyle().
    Bold(true).Foreground(lipgloss.Color("#F849A3")).Underline(true)

	var bold2 = lipgloss.NewStyle().
    Bold(true).Foreground(lipgloss.Color("#F849A3"))

	var enter = lipgloss.NewStyle().Foreground(lipgloss.Color("#00F3CF")).Blink(true)

	var enter2 = lipgloss.NewStyle().Foreground(lipgloss.Color("#F849A3")).Blink(true)

	var text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Align(lipgloss.Center)


	var dull = lipgloss.NewStyle().Foreground(lipgloss.Color("#878B7D"))

	var renderedFlavorText = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			text.Render(title.Render("About:") + "\n" + bold2.Render("CodeCube") + " is a project i made in a few hours because i was bored\n and didn't really have anything else todo.\nMade with ❤️  by Max Campbell\n" + dull.Render("Press b to go back to the home page")),
		)

		var centerMenu = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			text.Render("Welcome To " + title.Render("CodeCube") + "\nThe place where you go to paste your " + bold.Render("code!") + "\n" + enter.Render("Press X to create a new Paste") + "\n" + enter2.Render("Press R to get a paste") + "\n" + dull.Render("Press a to learn more about this project")),
		)
		

	switch m.state {
		case "pasted":
			return fmt.Sprintf("🚀 Paste saved!\n" + bold.Render("ID: " + m.savedId))
		case "":
			return fmt.Sprintf(centerMenu)
		case "flavor":
			return fmt.Sprintf(renderedFlavorText)
		case "newPaste":
			return fmt.Sprintf(enter.Render("Paste your content below:") + "\n\n%s",m.textInput.View())
		case "copied":
			return fmt.Sprintf(bold2.Render("🚀 Copied to your clipboard!"))
		case "getPaste":
			return fmt.Sprintf(enter.Render("Enter paste ID:") + "\n\n%s",m.textInput.View())
		case "loading":
			return m.progress.View()
		default:
			return fmt.Sprintf(bold.Render("Uh oh..."))
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
