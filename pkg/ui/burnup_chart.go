package ui

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type BurnUpPoint struct {
	Date      time.Time
	DateLabel string
	Total     int
	Completed int
}

type BurnUpChart struct {
	Container *fyne.Container

	headerLabel *widget.Label
	statusLabel *widget.Label
	scopeLabel  *widget.Label
	doneLabel   *widget.Label
	statsWrap   *fyne.Container

	chartCanvas *burnUpPlotWidget
}

func NewBurnUpChart() *BurnUpChart {
	bc := &BurnUpChart{}

	bc.headerLabel = widget.NewLabelWithStyle("📈 Burn-Up Progress", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Blue indicator for Total Scope
	scopeBox := canvas.NewRectangle(color.NRGBA{R: 70, G: 130, B: 240, A: 255})
	scopeBox.CornerRadius = 3
	scopeIcon := container.NewGridWrap(fyne.NewSize(10, 10), scopeBox)
	bc.scopeLabel = widget.NewLabelWithStyle("Total: 0", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	legendScope := container.NewHBox(container.NewCenter(scopeIcon), bc.scopeLabel)

	// Green indicator for Completed
	doneBox := canvas.NewRectangle(color.NRGBA{R: 35, G: 195, B: 115, A: 255})
	doneBox.CornerRadius = 3
	doneIcon := container.NewGridWrap(fyne.NewSize(10, 10), doneBox)
	bc.doneLabel = widget.NewLabelWithStyle("Completed: 0", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	legendDone := container.NewHBox(container.NewCenter(doneIcon), bc.doneLabel)

	// Status badge wrapper
	initialBadge := MakeBadge("0/0 (0%)", color.NRGBA{R: 60, G: 65, B: 75, A: 255}, color.White)
	bc.statsWrap = container.NewStack(initialBadge)

	topRight := container.NewHBox(
		legendScope,
		widget.NewLabel("·"),
		legendDone,
		bc.statsWrap,
	)

	// Subtitle row that displays real-time point details on hover
	bc.statusLabel = widget.NewLabelWithStyle("Hover over any milestone to inspect date & task details", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	topBar := container.NewVBox(
		container.NewBorder(nil, nil, bc.headerLabel, topRight),
		bc.statusLabel,
	)

	bc.chartCanvas = newBurnUpPlotWidget(func(pt *BurnUpPoint) {
		if pt == nil {
			bc.statusLabel.SetText("Hover over any milestone to inspect date & task details")
			bc.statusLabel.TextStyle = fyne.TextStyle{Italic: true}
		} else {
			pending := pt.Total - pt.Completed
			bc.statusLabel.SetText(fmt.Sprintf("📅 %s:   ● Total Scope: %d   ·   ● Completed: %d   ·   ⏳ Pending: %d", pt.DateLabel, pt.Total, pt.Completed, pending))
			bc.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
		}
		bc.statusLabel.Refresh()
	})

	chartCard := container.NewBorder(
		topBar,
		nil,
		nil,
		nil,
		bc.chartCanvas,
	)

	bg := canvas.NewRectangle(color.NRGBA{R: 24, G: 28, B: 36, A: 255})
	bg.CornerRadius = 8

	bc.Container = container.NewStack(bg, container.NewPadded(chartCard))
	return bc
}

func (bc *BurnUpChart) Update(tasks []model.Task) {
	total := len(tasks)
	completed := 0
	for _, t := range tasks {
		if t.Done {
			completed++
		}
	}

	pct := 0
	if total > 0 {
		pct = int(math.Round(float64(completed) * 100.0 / float64(total)))
	}

	// Update text labels
	bc.scopeLabel.SetText(fmt.Sprintf("Total: %d", total))
	bc.doneLabel.SetText(fmt.Sprintf("Completed: %d", completed))

	badgeText := fmt.Sprintf("%d/%d (%d%%)", completed, total, pct)
	badgeColor := color.NRGBA{R: 60, G: 65, B: 75, A: 255}
	if pct == 100 && total > 0 {
		badgeColor = color.NRGBA{R: 30, G: 130, B: 75, A: 255}
	} else if completed > 0 {
		badgeColor = color.NRGBA{R: 45, G: 90, B: 150, A: 255}
	}

	newBadge := MakeBadge(badgeText, badgeColor, color.White)
	bc.statsWrap.Objects = []fyne.CanvasObject{newBadge}
	bc.statsWrap.Refresh()

	points := CalculateBurnUpPoints(tasks)
	bc.chartCanvas.setPoints(points, total, completed)
	bc.Container.Refresh()
}

func (bc *BurnUpChart) ChartWidget() fyne.CanvasObject {
	return bc.chartCanvas
}

// CalculateBurnUpPoints generates the chronological points for the burn-up chart
func CalculateBurnUpPoints(tasks []model.Task) []BurnUpPoint {
	if len(tasks) == 0 {
		now := time.Now()
		return []BurnUpPoint{
			{Date: now, DateLabel: now.Format("02 Jan"), Total: 0, Completed: 0},
		}
	}

	earliest := time.Now()
	for _, t := range tasks {
		if !t.InsertedAt.IsZero() && t.InsertedAt.Before(earliest) {
			earliest = t.InsertedAt
		}
	}

	now := time.Now()
	startDate := time.Date(earliest.Year(), earliest.Month(), earliest.Day(), 0, 0, 0, 0, earliest.Location())
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	daysDiff := int(endDate.Sub(startDate).Hours()/24) + 1
	if daysDiff < 1 {
		daysDiff = 1
	}

	step := 1
	if daysDiff > 14 {
		step = int(math.Ceil(float64(daysDiff) / 12.0))
	}

	var points []BurnUpPoint
	curr := startDate
	for !curr.After(endDate) {
		dayEnd := time.Date(curr.Year(), curr.Month(), curr.Day(), 23, 59, 59, 999999999, curr.Location())

		tot := 0
		done := 0
		for _, t := range tasks {
			if !t.InsertedAt.IsZero() && !t.InsertedAt.After(dayEnd) {
				tot++
			}
			if t.Done && t.TerminatedAt != nil && !t.TerminatedAt.After(dayEnd) {
				done++
			}
		}

		label := curr.Format("02 Jan")
		points = append(points, BurnUpPoint{
			Date:      curr,
			DateLabel: label,
			Total:     tot,
			Completed: done,
		})

		curr = curr.AddDate(0, 0, step)
	}

	// Always ensure current moment is represented at the end
	lastPoint := points[len(points)-1]
	currentTotal := len(tasks)
	currentDone := 0
	for _, t := range tasks {
		if t.Done {
			currentDone++
		}
	}
	if lastPoint.Total != currentTotal || lastPoint.Completed != currentDone {
		points = append(points, BurnUpPoint{
			Date:      now,
			DateLabel: now.Format("02 Jan"),
			Total:     currentTotal,
			Completed: currentDone,
		})
	}

	return points
}

// burnUpPlotWidget custom canvas widget for rendering lines, axes, values and hover tooltips
type burnUpPlotWidget struct {
	widget.BaseWidget
	points     []BurnUpPoint
	total      int
	completed  int
	hoverIndex int // -1 when not hovering
	onHover    func(pt *BurnUpPoint)
}

var _ desktop.Hoverable = (*burnUpPlotWidget)(nil)

func newBurnUpPlotWidget(onHover func(pt *BurnUpPoint)) *burnUpPlotWidget {
	w := &burnUpPlotWidget{
		hoverIndex: -1,
		onHover:    onHover,
	}
	w.ExtendBaseWidget(w)
	return w
}

func (w *burnUpPlotWidget) setPoints(pts []BurnUpPoint, total, completed int) {
	w.points = pts
	w.total = total
	w.completed = completed
	w.hoverIndex = -1
	if w.onHover != nil {
		w.onHover(nil)
	}
	w.Refresh()
}

func (w *burnUpPlotWidget) MouseIn(e *desktop.MouseEvent) {
	w.updateHover(e.Position)
}

func (w *burnUpPlotWidget) MouseMoved(e *desktop.MouseEvent) {
	w.updateHover(e.Position)
}

func (w *burnUpPlotWidget) MouseOut() {
	if w.hoverIndex != -1 {
		w.hoverIndex = -1
		if w.onHover != nil {
			w.onHover(nil)
		}
		w.Refresh()
	}
}

func (w *burnUpPlotWidget) Tapped(e *fyne.PointEvent) {
	w.updateHover(e.Position)
}

func (w *burnUpPlotWidget) updateHover(pos fyne.Position) {
	n := len(w.points)
	if n <= 0 {
		return
	}
	size := w.Size()
	padLeft := float32(32)
	padRight := float32(28)
	plotW := size.Width - padLeft - padRight
	if plotW <= 0 {
		return
	}

	ratio := (pos.X - padLeft) / plotW
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	idx := int(math.Round(float64(ratio) * float64(n-1)))
	if idx >= 0 && idx < n {
		if idx != w.hoverIndex {
			w.hoverIndex = idx
			if w.onHover != nil {
				w.onHover(&w.points[idx])
			}
			w.Refresh()
		}
	}
}

func (w *burnUpPlotWidget) CreateRenderer() fyne.WidgetRenderer {
	return &burnUpRenderer{
		widget: w,
	}
}

type burnUpRenderer struct {
	widget  *burnUpPlotWidget
	objects []fyne.CanvasObject
}

func (r *burnUpRenderer) Destroy() {}

func (r *burnUpRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *burnUpRenderer) MinSize() fyne.Size {
	return fyne.NewSize(260, 120)
}

func (r *burnUpRenderer) Refresh() {
	r.Layout(r.widget.Size())
}

func (r *burnUpRenderer) Layout(size fyne.Size) {
	r.objects = nil

	if size.Width <= 10 || size.Height <= 10 {
		return
	}

	padLeft := float32(32)
	padBottom := float32(22)
	padTop := float32(14)
	padRight := float32(32)

	plotW := size.Width - padLeft - padRight
	plotH := size.Height - padTop - padBottom

	if plotW <= 10 || plotH <= 10 {
		return
	}

	maxVal := 5
	for _, p := range r.widget.points {
		if p.Total > maxVal {
			maxVal = p.Total
		}
	}
	headroom := int(math.Ceil(float64(maxVal) * 1.15))
	if headroom > maxVal {
		maxVal = headroom
	}

	// Horizontal grid lines and Y-axis labels
	gridColor := color.NRGBA{R: 44, G: 50, B: 62, A: 255}
	textMuted := color.NRGBA{R: 130, G: 138, B: 155, A: 255}

	steps := 3
	for i := 0; i <= steps; i++ {
		ratio := float32(i) / float32(steps)
		y := padTop + plotH*(1.0-ratio)
		val := int(math.Round(float64(ratio) * float64(maxVal)))

		// Grid line
		line := canvas.NewLine(gridColor)
		line.Position1 = fyne.NewPos(padLeft, y)
		line.Position2 = fyne.NewPos(padLeft+plotW, y)
		line.StrokeWidth = 1
		r.objects = append(r.objects, line)

		// Y label
		lbl := canvas.NewText(fmt.Sprintf("%d", val), textMuted)
		lbl.TextSize = 9
		lbl.Alignment = fyne.TextAlignTrailing
		lbl.Move(fyne.NewPos(padLeft-28, y-7))
		lbl.Resize(fyne.NewSize(24, 14))
		r.objects = append(r.objects, lbl)
	}

	// If no data
	if len(r.widget.points) <= 1 && r.widget.total == 0 {
		emptyLbl := canvas.NewText("Add tasks to track project burn-up progress", textMuted)
		emptyLbl.TextSize = 11
		emptyLbl.Alignment = fyne.TextAlignCenter
		emptyLbl.Move(fyne.NewPos(padLeft, padTop+plotH/2-8))
		emptyLbl.Resize(fyne.NewSize(plotW, 16))
		r.objects = append(r.objects, emptyLbl)
		return
	}

	n := len(r.widget.points)
	coordX := func(i int) float32 {
		if n <= 1 {
			return padLeft + plotW/2
		}
		return padLeft + (float32(i)/float32(n-1))*plotW
	}

	coordY := func(val int) float32 {
		ratio := float32(val) / float32(maxVal)
		return padTop + plotH*(1.0-ratio)
	}

	scopeColor := color.NRGBA{R: 70, G: 130, B: 240, A: 255}
	doneColor := color.NRGBA{R: 35, G: 195, B: 115, A: 255}

	// 1. Draw Scope line (Blue)
	for i := 0; i < n-1; i++ {
		p1 := fyne.NewPos(coordX(i), coordY(r.widget.points[i].Total))
		p2 := fyne.NewPos(coordX(i+1), coordY(r.widget.points[i+1].Total))

		line := canvas.NewLine(scopeColor)
		line.Position1 = p1
		line.Position2 = p2
		line.StrokeWidth = 2.5
		r.objects = append(r.objects, line)
	}

	// 2. Draw Done line (Green)
	for i := 0; i < n-1; i++ {
		p1 := fyne.NewPos(coordX(i), coordY(r.widget.points[i].Completed))
		p2 := fyne.NewPos(coordX(i+1), coordY(r.widget.points[i+1].Completed))

		line := canvas.NewLine(doneColor)
		line.Position1 = p1
		line.Position2 = p2
		line.StrokeWidth = 2.5
		r.objects = append(r.objects, line)
	}

	// 3. Draw Dots and direct numeric labels on points
	showAllNumbers := n <= 8
	for i, pt := range r.widget.points {
		cx := coordX(i)

		// Scope dot & value
		cyScope := coordY(pt.Total)
		dotScope := canvas.NewCircle(scopeColor)
		dotScope.Move(fyne.NewPos(cx-3.5, cyScope-3.5))
		dotScope.Resize(fyne.NewSize(7, 7))
		r.objects = append(r.objects, dotScope)

		// Done dot & value
		cyDone := coordY(pt.Completed)
		dotDone := canvas.NewCircle(doneColor)
		dotDone.Move(fyne.NewPos(cx-3.5, cyDone-3.5))
		dotDone.Resize(fyne.NewSize(7, 7))
		r.objects = append(r.objects, dotDone)

		// Numbers directly visible: on end points, or all points if <= 8
		if showAllNumbers || i == n-1 {
			txtScope := canvas.NewText(fmt.Sprintf("%d", pt.Total), scopeColor)
			txtScope.TextSize = 9.5
			txtScope.TextStyle = fyne.TextStyle{Bold: true}
			txtScope.Alignment = fyne.TextAlignCenter
			txtScope.Move(fyne.NewPos(cx-12, cyScope-16))
			txtScope.Resize(fyne.NewSize(24, 12))
			r.objects = append(r.objects, txtScope)

			txtDone := canvas.NewText(fmt.Sprintf("%d", pt.Completed), doneColor)
			txtDone.TextSize = 9.5
			txtDone.TextStyle = fyne.TextStyle{Bold: true}
			txtDone.Alignment = fyne.TextAlignCenter
			offsetY := float32(6)
			if cyDone == cyScope {
				offsetY = 16
			}
			txtDone.Move(fyne.NewPos(cx-12, cyDone+offsetY))
			txtDone.Resize(fyne.NewSize(24, 12))
			r.objects = append(r.objects, txtDone)
		}
	}

	// 4. X-Axis Date labels (first, middle, last)
	if n > 0 {
		dateLabels := []struct {
			index int
			align fyne.TextAlign
		}{
			{index: 0, align: fyne.TextAlignLeading},
		}
		if n >= 3 {
			dateLabels = append(dateLabels, struct {
				index int
				align fyne.TextAlign
			}{index: n / 2, align: fyne.TextAlignCenter})
		}
		if n >= 2 {
			dateLabels = append(dateLabels, struct {
				index int
				align fyne.TextAlign
			}{index: n - 1, align: fyne.TextAlignTrailing})
		}

		for _, dl := range dateLabels {
			pt := r.widget.points[dl.index]
			lbl := canvas.NewText(pt.DateLabel, textMuted)
			lbl.TextSize = 8.5
			lbl.Alignment = dl.align
			lblW := float32(60)
			posX := coordX(dl.index) - lblW/2
			if dl.align == fyne.TextAlignLeading {
				posX = coordX(dl.index)
			} else if dl.align == fyne.TextAlignTrailing {
				posX = coordX(dl.index) - lblW
			}
			lbl.Move(fyne.NewPos(posX, size.Height-padBottom+4))
			lbl.Resize(fyne.NewSize(lblW, 14))
			r.objects = append(r.objects, lbl)
		}
	}

	// 5. Interactive Hover Guide & Highlighted Point
	if r.widget.hoverIndex >= 0 && r.widget.hoverIndex < n {
		hi := r.widget.hoverIndex
		pt := r.widget.points[hi]
		hx := coordX(hi)

		// Vertical hairline
		hairline := canvas.NewLine(color.NRGBA{R: 110, G: 125, B: 155, A: 200})
		hairline.Position1 = fyne.NewPos(hx, padTop)
		hairline.Position2 = fyne.NewPos(hx, padTop+plotH)
		hairline.StrokeWidth = 1.5
		r.objects = append(r.objects, hairline)

		// Glowing halo rings on the hovered dots
		haloScope := canvas.NewCircle(color.NRGBA{R: 70, G: 130, B: 240, A: 140})
		haloScope.Move(fyne.NewPos(hx-6.5, coordY(pt.Total)-6.5))
		haloScope.Resize(fyne.NewSize(13, 13))
		r.objects = append(r.objects, haloScope)

		haloDone := canvas.NewCircle(color.NRGBA{R: 35, G: 195, B: 115, A: 140})
		haloDone.Move(fyne.NewPos(hx-6.5, coordY(pt.Completed)-6.5))
		haloDone.Resize(fyne.NewSize(13, 13))
		r.objects = append(r.objects, haloDone)

		// Highlighted numeric labels right at the hovered points
		txtScopeHover := canvas.NewText(fmt.Sprintf("%d", pt.Total), scopeColor)
		txtScopeHover.TextSize = 10.5
		txtScopeHover.TextStyle = fyne.TextStyle{Bold: true}
		txtScopeHover.Alignment = fyne.TextAlignCenter
		txtScopeHover.Move(fyne.NewPos(hx-15, coordY(pt.Total)-18))
		txtScopeHover.Resize(fyne.NewSize(30, 14))
		r.objects = append(r.objects, txtScopeHover)

		txtDoneHover := canvas.NewText(fmt.Sprintf("%d", pt.Completed), doneColor)
		txtDoneHover.TextSize = 10.5
		txtDoneHover.TextStyle = fyne.TextStyle{Bold: true}
		txtDoneHover.Alignment = fyne.TextAlignCenter
		offsetY := float32(6)
		if coordY(pt.Completed) == coordY(pt.Total) {
			offsetY = 16
		}
		txtDoneHover.Move(fyne.NewPos(hx-15, coordY(pt.Completed)+offsetY))
		txtDoneHover.Resize(fyne.NewSize(30, 14))
		r.objects = append(r.objects, txtDoneHover)
	}
}
