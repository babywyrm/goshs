package mytui

import "github.com/rivo/tview"

// DisplayTUI will display the UI
func DisplayTUI() {
	/* Simple box
	box := tview.NewBox().SetBorder(true).SetTitle("Testtitle!")
	if err := tview.NewApplication().SetRoot(box, true).Run(); err != nil {
		panic(err)
	}
	*/

	/* Working grid layout
	newPrimitive := func(text string) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetText(text)
	}
	header := newPrimitive("Header")
	files := newPrimitive("Files")
	clip := newPrimitive("Clipboard")
	footer := newPrimitive("Footer")

	grid := tview.NewGrid().
		SetRows(4, 0, 2).
		SetColumns(-2, -1).
		SetBorders(true).
		AddItem(header, 0, 0, 1, 2, 0, 0, false).
		AddItem(footer, 2, 0, 1, 2, 0, 0, false)

	// Layout for screens wider than 100 cells.
	grid.AddItem(files, 1, 0, 1, 1, 0, 100, false).
		AddItem(clip, 1, 1, 1, 1, 0, 100, false)

	if err := tview.NewApplication().SetRoot(grid, true).SetFocus(grid).Run(); err != nil {
		panic(err)
	}
	*/

	app := tview.NewApplication()

	flex := tview.NewFlex().AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewBox().SetBorder(true).SetTitle("Header"), 5, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(tview.NewBox().SetBorder(true).SetTitle("Files"), 0, 2, true).
			AddItem(tview.NewBox().SetBorder(true).SetTitle("Clipboard"), 0, 1, false), 0, 1, false).
		AddItem(tview.NewBox().SetBorder(true).SetTitle("Footer"), 5, 1, false), 0, 1, false)

	if err := app.SetRoot(flex, true).SetFocus(flex).Run(); err != nil {
		panic(err)
	}
}
