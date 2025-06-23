//go:build !cli

/*
 * SPDX-License-Identifier: GPL-3.0
 * Vencord Installer, a cross platform gui/cli app for installing Vencord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

import (
	"bytes"
	_ "embed"
	"errors"
	"image"
	"image/color"
	"vencord/buildinfo"

	g "github.com/AllenDang/giu"
	"github.com/AllenDang/imgui-go"

	// png decoder for icon
	_ "image/png"
	// jpeg decoder for background
	_ "image/jpeg"
	"os"
	path "path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	discords        []any
	radioIdx        int
	customChoiceIdx int

	customDir              string
	autoCompleteDir        string
	autoCompleteFile       string
	autoCompleteCandidates []string
	autoCompleteIdx        int
	lastAutoComplete       string
	didAutoComplete        bool

	modalId      = 0
	modalTitle   = "Oh Non :("
	modalMessage = "Vous ne devriez jamais voir ceci"

	acceptedOpenAsar   bool
	showedUpdatePrompt bool

	win *g.MasterWindow
)

//go:embed winres/icon.png
var iconBytes []byte

// Couleurs inspirées de Zelda Majora's Mask
var (
	ZeldaDarkPurple = color.RGBA{R: 0x2A, G: 0x1B, B: 0x3D, A: 0xFF}
	ZeldaGold       = color.RGBA{R: 0xFF, G: 0xD7, B: 0x00, A: 0xFF}
	ZeldaDeepBlue   = color.RGBA{R: 0x1A, G: 0x2B, B: 0x4A, A: 0xFF}
)

func init() {
	LogLevel = LevelDebug
}

func main() {
	InitGithubDownloader()
	discords = FindDiscords()

	customChoiceIdx = len(discords)

	go func() {
		<-GithubDoneChan
		g.Update()
	}()

	go func() {
		<-SelfUpdateCheckDoneChan
		g.Update()
	}()

	var linuxFlags g.MasterWindowFlags = 0
	if runtime.GOOS == "linux" {
		os.Setenv("GDK_SCALE", "1")
		os.Setenv("GDK_DPI_SCALE", "1")
	}

	win = g.NewMasterWindow("Bashcord", 1400, 900, linuxFlags)

	icon, _, err := image.Decode(bytes.NewReader(iconBytes))
	if err != nil {
		Log.Warn("Failed to load application icon", err)
		Log.Debug(iconBytes, len(iconBytes))
	} else {
		win.SetIcon([]image.Image{icon})
	}

	win.Run(loop)
}

type CondWidget struct {
	predicate  bool
	ifWidget   func() g.Widget
	elseWidget func() g.Widget
}

func (w *CondWidget) Build() {
	if w.predicate {
		w.ifWidget().Build()
	} else if w.elseWidget != nil {
		w.elseWidget().Build()
	}
}

func getChosenInstall() *DiscordInstall {
	var choice *DiscordInstall
	if radioIdx == customChoiceIdx {
		choice = ParseDiscord(customDir, "")
		if choice == nil {
			g.OpenPopup("#invalid-custom-location")
		}
	} else {
		choice = discords[radioIdx].(*DiscordInstall)
	}
	return choice
}

func InstallLatestBuilds() (err error) {
	if IsDevInstall {
		return
	}

	err = installLatestBuilds()
	if err != nil {
		ShowModal("Oups !", "Échec de l'installation des dernières versions de Bashcord depuis GitHub :\n"+err.Error())
	}
	return
}

func handlePatch() {
	choice := getChosenInstall()
	if choice != nil {
		choice.Patch()
	}
}

func handleUnpatch() {
	choice := getChosenInstall()
	if choice != nil {
		choice.Unpatch()
	}
}

func handleOpenAsar() {
	if acceptedOpenAsar || getChosenInstall().IsOpenAsar() {
		handleOpenAsarConfirmed()
		return
	}

	g.OpenPopup("#openasar-confirm")
}

func handleOpenAsarConfirmed() {
	choice := getChosenInstall()
	if choice != nil {
		if choice.IsOpenAsar() {
			if err := choice.UninstallOpenAsar(); err != nil {
				handleErr(choice, err, "désinstaller OpenAsar de")
			} else {
				g.OpenPopup("#openasar-unpatched")
				g.Update()
			}
		} else {
			if err := choice.InstallOpenAsar(); err != nil {
				handleErr(choice, err, "installer OpenAsar sur")
			} else {
				g.OpenPopup("#openasar-patched")
				g.Update()
			}
		}
	}
}

func handleErr(di *DiscordInstall, err error, action string) {
	if errors.Is(err, os.ErrPermission) {
		switch runtime.GOOS {
		case "windows":
			err = errors.New("Permission refusée. Assurez-vous que Discord est complètement fermé (depuis la barre système) !")
		case "darwin":
			// FIXME: This text is not selectable which is a bit mehhh
			command := "sudo chown -R \"${USER}:wheel\" " + di.path
			err = errors.New("Permission refusée. Veuillez accorder à l'installateur l'accès complet au disque dans les paramètres système (page confidentialité et sécurité).\n\nSi cela ne fonctionne toujours pas, essayez d'exécuter la commande suivante dans votre terminal :\n" + command)
		case "linux":
			command := "sudo chown -R \"$USER:$USER\" " + di.path
			err = errors.New("Permission refusée. Essayez d'exécuter l'installateur avec les privilèges sudo.\n\nSi cela ne fonctionne toujours pas, essayez d'exécuter la commande suivante dans votre terminal :\n" + command)
		default:
			err = errors.New("Permission refusée. Essayez peut-être de m'exécuter en tant qu'Administrateur/Root ?")
		}
	}

	ShowModal("Échec de "+action+" cette installation", err.Error())
}

func HandleScuffedInstall() {
	g.OpenPopup("#scuffed-install")
}

func (di *DiscordInstall) Patch() {
	if CheckScuffedInstall() {
		return
	}
	if err := di.patch(); err != nil {
		handleErr(di, err, "patcher")
	} else {
		g.OpenPopup("#patched")
	}
}

func (di *DiscordInstall) Unpatch() {
	if err := di.unpatch(); err != nil {
		handleErr(di, err, "dépatcher")
	} else {
		g.OpenPopup("#unpatched")
	}
}

func onCustomInputChanged() {
	p := customDir
	if len(p) != 0 {
		// Select the custom option for people
		radioIdx = customChoiceIdx
	}

	dir := path.Dir(p)

	isNewDir := strings.HasSuffix(p, "/")
	wentUpADir := !isNewDir && dir != autoCompleteDir

	if isNewDir || wentUpADir {
		autoCompleteDir = dir
		// reset all the funnies
		autoCompleteIdx = 0
		lastAutoComplete = ""
		autoCompleteFile = ""
		autoCompleteCandidates = nil

		// Generate autocomplete items
		files, err := os.ReadDir(dir)
		if err == nil {
			for _, file := range files {
				autoCompleteCandidates = append(autoCompleteCandidates, file.Name())
			}
		}
	} else if !didAutoComplete {
		// reset auto complete and update our file
		autoCompleteFile = path.Base(p)
		lastAutoComplete = ""
	}

	if wentUpADir {
		autoCompleteFile = path.Base(p)
	}

	didAutoComplete = false
}

// go can you give me []any?
// to pass to giu RangeBuilder?
// yeeeeees
// actually returns []string like a boss
func makeAutoComplete() []any {
	input := strings.ToLower(autoCompleteFile)

	var candidates []any
	for _, e := range autoCompleteCandidates {
		file := strings.ToLower(e)
		if autoCompleteFile == "" || strings.HasPrefix(file, input) {
			candidates = append(candidates, e)
		}
	}
	return candidates
}

func makeRadioOnChange(i int) func() {
	return func() {
		radioIdx = i
	}
}

func Tooltip(label string) g.Widget {
	return g.Style().
		SetStyle(g.StyleVarWindowPadding, 10, 8).
		SetStyleFloat(g.StyleVarWindowRounding, 8).
		To(
			g.Tooltip(label),
		)
}

func InfoModal(id, title, description string) g.Widget {
	return RawInfoModal(id, title, description, false)
}

func RawInfoModal(id, title, description string, isOpenAsar bool) g.Widget {
	isDynamic := strings.HasPrefix(id, "#modal") && !strings.Contains(description, "\n")
	return g.Style().
		SetStyle(g.StyleVarWindowPadding, 30, 30).
		SetStyleFloat(g.StyleVarWindowRounding, 12).
		To(
			g.PopupModal(id).
				Flags(g.WindowFlagsNoTitleBar | Ternary(isDynamic, g.WindowFlagsAlwaysAutoResize, 0)).
				Layout(
					g.Align(g.AlignCenter).To(
						g.Style().SetFontSize(30).To(
							g.Label(title),
						),
						g.Style().SetFontSize(20).To(
							g.Label(description).Wrapped(isDynamic),
						),
						&CondWidget{id == "#scuffed-install", func() g.Widget {
							return g.Column(
								g.Dummy(0, 10),
								g.Button("Emmène-moi là !").OnClick(func() {
									// this issue only exists on windows so using Windows specific path is oki
									username := os.Getenv("USERNAME")
									programData := os.Getenv("PROGRAMDATA")
									g.OpenURL("file://" + path.Join(programData, username))
								}).Size(200, 30),
							)
						}, nil},
						g.Dummy(0, 20),
						&CondWidget{isOpenAsar,
							func() g.Widget {
								return g.Row(
									g.Button("Accepter").
										OnClick(func() {
											acceptedOpenAsar = true
											g.CloseCurrentPopup()
										}).
										Size(100, 30),
									g.Button("Annuler").
										OnClick(func() {
											g.CloseCurrentPopup()
										}).
										Size(100, 30),
								)
							},
							func() g.Widget {
								return g.Button("Ok").
									OnClick(func() {
										g.CloseCurrentPopup()
									}).
									Size(100, 30)
							},
						},
					),
				),
		)
}

func UpdateModal() g.Widget {
	return g.Style().
		SetStyle(g.StyleVarWindowPadding, 30, 30).
		SetStyleFloat(g.StyleVarWindowRounding, 12).
		To(
			g.PopupModal("#update-prompt").
				Flags(g.WindowFlagsNoTitleBar | g.WindowFlagsAlwaysAutoResize).
				Layout(
					g.Align(g.AlignCenter).To(
						g.Style().SetFontSize(30).To(
							g.Label("Votre installateur est obsolète !"),
						),
						g.Style().SetFontSize(20).To(
							g.Label(
								"Souhaitez-vous mettre à jour maintenant ?\n\n"+
									"Une fois que vous appuyez sur Mettre à jour maintenant, le nouvel installateur sera automatiquement téléchargé.\n"+
									"L'installateur semblera temporairement ne plus répondre. Attendez simplement !\n"+
									"Une fois la mise à jour terminée, l'installateur se rouvrira automatiquement.\n\n"+
									"Sur MacOS, les mises à jour automatiques ne sont pas prises en charge, il s'ouvrira donc dans le navigateur.",
							),
						),
						g.Row(
							g.Button("Mettre à jour maintenant").
								OnClick(func() {
									if runtime.GOOS == "darwin" {
										g.CloseCurrentPopup()
										g.OpenURL(GetInstallerDownloadLink())
										return
									}

									err := UpdateSelf()
									g.CloseCurrentPopup()

									if err != nil {
										ShowModal("Échec de la mise à jour automatique !", err.Error())
									} else {
										if err = RelaunchSelf(); err != nil {
											ShowModal("Échec du redémarrage automatique ! Veuillez le faire manuellement.", err.Error())
										}
									}
								}).
								Size(150, 30),
							g.Button("Plus tard").
								OnClick(func() {
									g.CloseCurrentPopup()
								}).
								Size(100, 30),
						),
					),
				),
		)
}

func ShowModal(title, desc string) {
	modalTitle = title
	modalMessage = desc
	modalId++
	g.OpenPopup("#modal" + strconv.Itoa(modalId))
}

func renderInstaller() g.Widget {
	candidates := makeAutoComplete()
	wi, _ := win.GetSize()
	w := float32(wi) - 96

	var currentDiscord *DiscordInstall
	if radioIdx != customChoiceIdx {
		currentDiscord = discords[radioIdx].(*DiscordInstall)
	}
	var isOpenAsar = currentDiscord != nil && currentDiscord.IsOpenAsar()

	if CanUpdateSelf() && !showedUpdatePrompt {
		showedUpdatePrompt = true
		g.OpenPopup("#update-prompt")
	}

	layout := g.Layout{
		g.Dummy(0, 20),
		g.Style().
			SetColor(g.StyleColorSeparator, ZeldaGold).
			To(
				g.Separator(),
			),
		g.Dummy(0, 5),

		g.Style().SetFontSize(20).To(
			renderErrorCard(
				ZeldaDarkPurple,
				"**Github** est le seul endroit officiel pour obtenir Bashcord. Tout autre site pretendant etre nous est malveillant.\n"+
					"Si vous avez telecharge depuis une autre source, vous devriez tout supprimer/desinstaller immediatement, effectuer une analyse anti-malware et changer votre mot de passe Discord.",
				90,
			),
		),

		g.Dummy(0, 5),

		g.Style().
			SetColor(g.StyleColorText, ZeldaGold).
			SetFontSize(30).
			To(
				g.Label("Veuillez selectionner une installation a patcher"),
			),

		&CondWidget{len(discords) == 0, func() g.Widget {
			s := "Aucune installation Discord trouvee. Vous devez d'abord installer Discord."
			if runtime.GOOS == "linux" {
				s += " snap n'est pas pris en charge."
			}
			return g.Style().
				SetColor(g.StyleColorText, color.RGBA{255, 100, 100, 255}).
				To(
					g.Label(s),
				)
		}, nil},

		g.Style().
			SetColor(g.StyleColorText, color.RGBA{255, 255, 255, 255}).
			SetFontSize(20).
			To(
				g.RangeBuilder("Discords", discords, func(i int, v any) g.Widget {
					d := v.(*DiscordInstall)
					//goland:noinspection GoDeprecation
					text := strings.Title(d.branch) + " - " + d.path
					if d.isPatched {
						text += " [PATCHE]"
					}
					return g.Style().
						SetColor(g.StyleColorCheckMark, ZeldaGold).
						To(
							g.RadioButton(text, radioIdx == i).
								OnChange(makeRadioOnChange(i)),
						)
				}),

				g.Style().
					SetColor(g.StyleColorCheckMark, ZeldaGold).
					To(
						g.RadioButton("Emplacement d'installation personnalise", radioIdx == customChoiceIdx).
							OnChange(makeRadioOnChange(customChoiceIdx)),
					),
			),

		g.Dummy(0, 5),
		g.Style().
			SetStyle(g.StyleVarFramePadding, 16, 16).
			SetColor(g.StyleColorFrameBg, ZeldaDarkPurple).
			SetColor(g.StyleColorFrameBgHovered, ZeldaGold).
			SetColor(g.StyleColorFrameBgActive, ZeldaGold).
			SetColor(g.StyleColorText, color.RGBA{255, 255, 255, 255}).
			SetFontSize(20).
			To(
				g.InputText(&customDir).Hint("L'emplacement personnalise").
					Size(w - 16).
					Flags(g.InputTextFlagsCallbackCompletion).
					OnChange(onCustomInputChanged).
					// this library has its own autocomplete but it's broken
					Callback(
						func(data imgui.InputTextCallbackData) int32 {
							if len(candidates) == 0 {
								return 0
							}
							// just wrap around
							if autoCompleteIdx >= len(candidates) {
								autoCompleteIdx = 0
							}

							// used by change handler
							didAutoComplete = true

							start := len(customDir)
							// Delete previous auto complete
							if lastAutoComplete != "" {
								start -= len(lastAutoComplete)
								data.DeleteBytes(start, len(lastAutoComplete))
							} else if autoCompleteFile != "" { // delete partial input
								start -= len(autoCompleteFile)
								data.DeleteBytes(start, len(autoCompleteFile))
							}

							// Insert auto complete
							lastAutoComplete = candidates[autoCompleteIdx].(string)
							data.InsertBytes(start, []byte(lastAutoComplete))
							autoCompleteIdx++

							return 0
						},
					),
			),
		g.Style().
			SetColor(g.StyleColorText, color.RGBA{200, 200, 255, 255}).
			To(
				g.RangeBuilder("AutoComplete", candidates, func(i int, v any) g.Widget {
					dir := v.(string)
					return g.Label(dir)
				}),
			),

		g.Dummy(0, 20),

		g.Style().SetFontSize(20).To(
			g.Row(
				g.Style().
					SetColor(g.StyleColorButton, ZeldaGold).
					SetColor(g.StyleColorButtonHovered, color.RGBA{255, 215, 0, 200}).
					SetColor(g.StyleColorButtonActive, color.RGBA{255, 215, 0, 255}).
					SetColor(g.StyleColorText, ZeldaDeepBlue).
					SetDisabled(GithubError != nil).
					To(
						g.Button("Installer").
							OnClick(handlePatch).
							Size((w-40)/4, 50),
						Tooltip("Patcher l'installation Discord selectionnee"),
					),
				g.Style().
					SetColor(g.StyleColorButton, color.RGBA{100, 149, 237, 255}).
					SetColor(g.StyleColorButtonHovered, color.RGBA{100, 149, 237, 200}).
					SetColor(g.StyleColorButtonActive, color.RGBA{100, 149, 237, 255}).
					SetColor(g.StyleColorText, color.RGBA{255, 255, 255, 255}).
					SetDisabled(GithubError != nil).
					To(
						g.Button("Reinstaller / Reparer").
							OnClick(func() {
								if IsDevInstall {
									handlePatch()
								} else {
									err := InstallLatestBuilds()
									if err == nil {
										handlePatch()
									}
								}
							}).
							Size((w-40)/4, 50),
						Tooltip("Reinstaller et mettre a jour Bashcord"),
					),
				g.Style().
					SetColor(g.StyleColorButton, color.RGBA{220, 20, 60, 255}).
					SetColor(g.StyleColorButtonHovered, color.RGBA{220, 20, 60, 200}).
					SetColor(g.StyleColorButtonActive, color.RGBA{220, 20, 60, 255}).
					SetColor(g.StyleColorText, color.RGBA{255, 255, 255, 255}).
					To(
						g.Button("Desinstaller").
							OnClick(handleUnpatch).
							Size((w-40)/4, 50),
						Tooltip("Depatcher l'installation Discord selectionnee"),
					),
				g.Style().
					SetColor(g.StyleColorButton, Ternary(isOpenAsar, color.RGBA{220, 20, 60, 255}, ZeldaGold)).
					SetColor(g.StyleColorButtonHovered, Ternary(isOpenAsar, color.RGBA{220, 20, 60, 200}, color.RGBA{255, 215, 0, 200})).
					SetColor(g.StyleColorButtonActive, Ternary(isOpenAsar, color.RGBA{220, 20, 60, 255}, color.RGBA{255, 215, 0, 255})).
					SetColor(g.StyleColorText, Ternary(isOpenAsar, color.RGBA{255, 255, 255, 255}, ZeldaDeepBlue)).
					To(
						g.Button(Ternary(isOpenAsar, "Desinstaller OpenAsar", Ternary(currentDiscord != nil, "Installer OpenAsar", "(Des)installer OpenAsar"))).
							OnClick(handleOpenAsar).
							Size((w-40)/4, 50),
						Tooltip("Gerer OpenAsar"),
					),
			),
		),

		InfoModal("#patched", "Patché avec succès", "Si Discord est encore ouvert, fermez-le complètement d'abord.\n"+
			"Ensuite, démarrez-le et vérifiez que Bashcord s'est installé avec succès en cherchant sa catégorie dans les Paramètres Discord"),
		InfoModal("#unpatched", "Dépatché avec succès", "Si Discord est encore ouvert, fermez-le complètement d'abord. Ensuite redémarrez-le, il devrait être revenu à l'état d'origine !"),
		InfoModal("#scuffed-install", "Attendez !", "Vous avez une installation Discord cassée.\n"+
			"Parfois Discord décide de s'installer au mauvais endroit pour une raison quelconque !\n"+
			"Vous devez corriger cela avant de patcher, sinon Bashcord ne fonctionnera probablement pas.\n\n"+
			"Utilisez le bouton ci-dessous pour y aller et supprimer tout dossier appelé Discord ou Squirrel.\n"+
			"Si le dossier est maintenant vide, n'hésitez pas à revenir en arrière et supprimer ce dossier aussi.\n"+
			"Ensuite voyez si Discord démarre toujours. Sinon, réinstallez-le"),
		RawInfoModal("#openasar-confirm", "OpenAsar", "OpenAsar est une alternative open-source de l'app.asar du bureau Discord.\n"+
			"Bashcord n'est en aucun cas affilié à OpenAsar.\n"+
			"Vous installez OpenAsar à vos propres risques. Si vous rencontrez des problèmes avec OpenAsar,\n"+
			"aucun support ne sera fourni, rejoignez plutôt le serveur OpenAsar !\n\n"+
			"Pour installer OpenAsar, appuyez sur Accepter et cliquez à nouveau sur 'Installer OpenAsar'.", true),
		InfoModal("#openasar-patched", "OpenAsar installé avec succès", "Si Discord est encore ouvert, fermez-le complètement d'abord. Ensuite redémarrez-le et vérifiez qu'OpenAsar s'est installé avec succès !"),
		InfoModal("#openasar-unpatched", "OpenAsar désinstallé avec succès", "Si Discord est encore ouvert, fermez-le complètement d'abord. Ensuite redémarrez-le et il devrait être revenu à l'état d'origine !"),
		InfoModal("#invalid-custom-location", "Emplacement invalide", "L'emplacement spécifié n'est pas une installation Discord valide.\nAssurez-vous de sélectionner le dossier de base.\n\nAstuce : Discord snap n'est pas pris en charge. utilisez flatpak ou .deb"),
		InfoModal("#modal"+strconv.Itoa(modalId), modalTitle, modalMessage),

		UpdateModal(),
	}

	return layout
}

func renderErrorCard(col color.Color, message string, height float32) g.Widget {
	return g.Style().
		SetColor(g.StyleColorChildBg, col).
		SetStyleFloat(g.StyleVarAlpha, 0.9).
		SetStyle(g.StyleVarWindowPadding, 10, 10).
		SetStyleFloat(g.StyleVarChildRounding, 5).
		To(
			g.Child().
				Size(g.Auto, height).
				Layout(
					g.Row(
						g.Style().SetColor(g.StyleColorText, color.Black).To(
							g.Markdown(&message),
						),
					),
				),
		)
}

func BackgroundImage() g.Widget {
	return g.Style().
		SetColor(g.StyleColorWindowBg, ZeldaDeepBlue).
		SetStyleFloat(g.StyleVarAlpha, 0.95).
		To(
			g.Dummy(0, 0),
		)
}

func loop() {
	g.PushWindowPadding(48, 48)

	g.SingleWindow().
		RegisterKeyboardShortcuts(
			g.WindowShortcut{Key: g.KeyUp, Callback: func() {
				if radioIdx > 0 {
					radioIdx--
				}
			}},
			g.WindowShortcut{Key: g.KeyDown, Callback: func() {
				if radioIdx < customChoiceIdx {
					radioIdx++
				}
			}},
		).
		Layout(
			// Appliquer le thème Zelda
			g.Style().
				SetColor(g.StyleColorWindowBg, ZeldaDeepBlue).
				SetColor(g.StyleColorChildBg, ZeldaDarkPurple).
				SetColor(g.StyleColorFrameBg, ZeldaDarkPurple).
				SetColor(g.StyleColorFrameBgHovered, ZeldaGold).
				SetColor(g.StyleColorFrameBgActive, ZeldaGold).
				SetColor(g.StyleColorCheckMark, ZeldaGold).
				To(
					g.Align(g.AlignCenter).To(
						g.Style().
							SetColor(g.StyleColorText, ZeldaGold).
							SetFontSize(45).
							To(
								g.Label("BASHCORD"),
							),
						g.Style().
							SetColor(g.StyleColorText, color.RGBA{200, 200, 255, 255}).
							SetFontSize(16).
							To(
								g.Label("~ Inspire par l'univers de Majora's Mask ~"),
							),
					),

					g.Dummy(0, 20),
					g.Style().
						SetColor(g.StyleColorText, color.RGBA{255, 255, 255, 255}).
						SetFontSize(20).
						To(
							g.Row(
								g.Label(Ternary(IsDevInstall, "Installation de developpement : ", "Bashcord sera telecharge vers : ")+EquicordDirectory),
								g.Style().
									SetColor(g.StyleColorButton, ZeldaGold).
									SetColor(g.StyleColorButtonHovered, ZeldaDarkPurple).
									SetColor(g.StyleColorButtonActive, ZeldaDarkPurple).
									SetColor(g.StyleColorText, ZeldaDeepBlue).
									SetStyle(g.StyleVarFramePadding, 4, 4).
									To(
										g.Button("Ouvrir le repertoire").OnClick(func() {
											g.OpenURL("file://" + path.Dir(EquicordDirectory))
										}),
									),
							),
							&CondWidget{!IsDevInstall, func() g.Widget {
								return g.Style().
									SetColor(g.StyleColorText, color.RGBA{200, 200, 255, 255}).
									To(
										g.Label("Pour personnaliser cet emplacement, definissez la variable d'environnement 'BASHCORD_USER_DATA_DIR' et redemarrez-moi").Wrapped(true),
									)
							}, nil},
							g.Dummy(0, 10),
							g.Style().
								SetColor(g.StyleColorText, color.RGBA{180, 180, 255, 255}).
								To(
									g.Label("Version de Bashcord : "+buildinfo.InstallerTag+" ("+buildinfo.InstallerGitHash+")"+Ternary(IsSelfOutdated, " - OBSOLETE", "")),
									g.Label("Version locale de Bashcord : "+InstalledHash),
								),
							&CondWidget{
								GithubError == nil,
								func() g.Widget {
									if IsDevInstall {
										return g.Style().
											SetColor(g.StyleColorText, color.RGBA{255, 200, 100, 255}).
											To(
												g.Label("Pas de mise a jour de Bashcord car en mode developpement"),
											)
									}
									return g.Style().
										SetColor(g.StyleColorText, color.RGBA{100, 255, 100, 255}).
										To(
											g.Label("Derniere version de Bashcord : " + LatestHash),
										)
								}, func() g.Widget {
									return renderErrorCard(DiscordRed, "Echec de recuperation des informations depuis GitHub : "+GithubError.Error(), 40)
								},
							},
						),

					renderInstaller(),
				),
		)

	g.PopStyle()
}
