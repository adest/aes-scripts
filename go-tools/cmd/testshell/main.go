package main

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/textinput"
)

// ─── 1. LES COMMANDES DISPONIBLES ────────────────────────────────────────────

// Une "map" en Go c'est un dictionnaire : clé → valeur.
// Ici : nom de commande → description.
// C'est pratique pour l'autocomplétion et le dispatch.
var commands = map[string]string{
    "hello": "dit bonjour",
    "add":   "additionne deux nombres",
    "exit":  "quitter le shell",
	"history": "show command history",
}

// ─── 2. LE MODEL ─────────────────────────────────────────────────────────────

// Une "struct" en Go, c'est comme un objet sans méthodes : juste des champs.
// C'est notre état global de l'application.
type model struct {
    input      textinput.Model // le composant champ de texte
    output     string          // la dernière ligne de résultat à afficher
    history    []string        // toutes les lignes précédentes (historique)
    suggestion string          // suggestion d'autocomplétion en cours
}

// "func" déclare une fonction.
// "initialModel() model" signifie : fonction sans argument, qui retourne un model.
func initialModel() model {
    // On crée et configure le composant textinput
    ti := textinput.New()
    ti.Placeholder = "tape une commande..."
    ti.Focus()              // il reçoit le focus clavier dès le départ
    ti.CharLimit = 100

    // On retourne un model initialisé
    // En Go, on peut initialiser une struct en nommant ses champs
    return model{
        input:   ti,
        output:  "Bienvenue ! Commandes disponibles : hello, add, exit",
        history: []string{},
    }
}

// ─── 3. INIT ─────────────────────────────────────────────────────────────────

// "Init" est appelé une seule fois au démarrage.
// Il peut lancer des effets asynchrones (réseau, timer...).
// Ici on n'en a pas besoin, donc on retourne nil.
//
// "(m model)" est le "receiver" : c'est comme "self" ou "this" en Python/JS.
// Ça permet d'attacher cette fonction au type model.
func (m model) Init() tea.Cmd {
    return textinput.Blink // fait clignoter le curseur
}

// ─── 4. UPDATE ───────────────────────────────────────────────────────────────

// Update reçoit un message (événement) et retourne :
//   - le nouveau model (potentiellement modifié)
//   - une commande optionnelle à exécuter ensuite (ou nil)
//
// "tea.Msg" est une interface vide : n'importe quel type peut être un message.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

    // "switch" en Go peut switcher sur le TYPE d'une valeur, pas juste sa valeur.
    // C'est ce qu'on appelle un "type switch". Très pratique pour gérer des événements.
    switch msg := msg.(type) {

    // Événement clavier
    case tea.KeyMsg:
        switch msg.Type {

        // Tab : autocomplétion
        case tea.KeyTab:
            if m.suggestion != "" {
                // On remplace le texte saisi par la suggestion
                m.input.SetValue(m.suggestion)
                // On déplace le curseur à la fin
                m.input.CursorEnd()
                m.suggestion = ""
            }

        // Entrée : exécuter la commande
        case tea.KeyEnter:
            raw := strings.TrimSpace(m.input.Value())
            if raw == "" {
                return m, nil
            }

            // "strings.Fields" découpe une string sur les espaces, et retourne
            // un slice ([]string). Un slice en Go, c'est un tableau dynamique.
            parts := strings.Fields(raw)

            // En Go, parts[0] est le premier élément, parts[1:] est le reste
            // (un "sous-slice" à partir de l'index 1).
            cmd, args := parts[0], parts[1:]

            // Commande spéciale : quitter
            if cmd == "exit" {
                return m, tea.Quit
            }

            // On exécute la commande et on récupère le résultat
            result := dispatch(cmd, args, m)

            // On ajoute la ligne saisie à l'historique
            m.history = append(m.history, "> "+raw)
            m.history = append(m.history, result)

            // On vide le champ de saisie
            m.input.SetValue("")
            m.suggestion = ""
            m.output = result

        // Ctrl+C ou Echap : quitter
        case tea.KeyCtrlC, tea.KeyEsc:
            return m, tea.Quit

        // N'importe quelle autre touche : mettre à jour le champ ET calculer la suggestion
        default:
            // On laisse textinput gérer l'événement clavier en premier
            var cmd tea.Cmd
            m.input, cmd = m.input.Update(msg)

            // Puis on calcule la suggestion d'autocomplétion
            m.suggestion = autocomplete(m.input.Value())

            return m, cmd
        }
    }

    // Pour tous les autres types de messages (resize fenêtre, etc.),
    // on laisse textinput les gérer
    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)
    return m, cmd
}

// ─── 5. VIEW ─────────────────────────────────────────────────────────────────

// View retourne une string : tout ce qui sera affiché à l'écran.
// Bubbletea se charge d'effacer et de réécrire à chaque update.
func (m model) View() string {
    // "var sb strings.Builder" déclare une variable de type strings.Builder.
    // C'est un buffer de texte efficace pour construire des strings morceau par morceau.
    var sb strings.Builder

    sb.WriteString("=== Mon Shell ===\n\n")

    // On affiche l'historique des commandes précédentes
    for _, line := range m.history {
        // "range" en Go permet d'itérer sur un slice.
        // Il retourne l'index et la valeur. Le "_" signifie "j'ignore l'index".
        sb.WriteString(line + "\n")
    }

    // Le prompt actuel
    sb.WriteString("\n> " + m.input.View())

    // Affichage de la suggestion en grisé si elle existe
    if m.suggestion != "" && m.suggestion != m.input.Value() {
        remaining := strings.TrimPrefix(m.suggestion, m.input.Value())
        sb.WriteString("\n  ↑ Tab pour compléter : " + m.suggestion + " (" + remaining + " manquant)")
    }

    sb.WriteString("\n\n[Tab] compléter  [Entrée] exécuter  [Ctrl+C] quitter\n")

    return sb.String()
}

// ─── 6. DISPATCH ─────────────────────────────────────────────────────────────

// dispatch reçoit le nom de la commande et ses arguments, et retourne un string à afficher.
// "[]string" = slice de strings (tableau dynamique de strings)
func dispatch(cmd string, args []string, m model) string {
    switch cmd {
    case "hello":
        // "fmt.Sprintf" formate une string comme printf, et la retourne.
        return fmt.Sprintf("Commande: %s | Args: %v → Bonjour !", cmd, args)

    case "add":
        return fmt.Sprintf("Commande: %s | Args: %v → (pour l'instant je ne calcule rien, mais je les vois !)", cmd, args)

    case "history":
		// concataine l'historique en separant par des sauts de ligne
		output := "Historique des commandes :\n"
		for i, line := range m.history {
			output += fmt.Sprintf("%d: %s\n", i+1, line)
		}
        return output

    default:
        return fmt.Sprintf("Commande inconnue : '%s'. Tape 'exit' pour quitter.", cmd)
    }
}

// ─── 7. AUTOCOMPLETE ─────────────────────────────────────────────────────────

// autocomplete cherche une commande qui commence par le texte saisi.
// Elle retourne la première correspondance trouvée, ou "" si rien.
func autocomplete(input string) string {
    if input == "" {
        return ""
    }
    // On itère sur la map commands
    // En Go, une map n'est pas ordonnée, mais ça suffit pour notre usage
    for name := range commands {
        if strings.HasPrefix(name, input) {
            return name
        }
    }
    return ""
}

// ─── 8. MAIN ─────────────────────────────────────────────────────────────────

func main() {
    // On crée le programme Bubbletea avec notre model initial
    p := tea.NewProgram(initialModel())

    // "p.Run()" lance la boucle principale (bloquante).
    // En Go, on gère les erreurs explicitement : pas d'exceptions.
    // "_" signifie "j'ignore la valeur de retour" (le model final ici).
    if _, err := p.Run(); err != nil {
        // "fmt.Printf" affiche sur stdout.
        // "%v" formate n'importe quelle valeur, "%s" pour les strings.
        fmt.Printf("Erreur : %v\n", err)
    }
}