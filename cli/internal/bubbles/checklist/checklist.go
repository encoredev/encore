package checklist

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

type Item interface {
	Render(selected, checked bool) string
}

type Model[I Item] struct {
	Data    []I
	PerPage int
	Initial []bool

	// init indicates whether the data model has completed initialization
	init bool

	checked []bool // len(checked) == len(pageData)

	// index global real time index
	index int
	// maxIndex global max index
	maxIndex int
	// pageIndex real time index of current page
	pageIndex int
	// pageMaxIndex current page max index
	pageMaxIndex int

	// pageData data set rendered in real time on the current page
	pageData []I
}

func (m Model[I]) Selected() (I, bool) {
	idx := m.pageIndex
	if idx >= 0 && idx < len(m.pageData) {
		return m.pageData[idx], true
	}
	var zero I
	return zero, false
}

func (m Model[I]) Checked() []I {
	indices := m.CheckedIndices()
	items := make([]I, len(indices))
	for i, idx := range indices {
		items[i] = m.pageData[idx]
	}
	return items
}

func (m Model[I]) CheckedIndices() []int {
	var indices []int
	for i, checked := range m.checked {
		if checked {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m Model[I]) View() string {
	var out strings.Builder
	cursor := "»" // TODO color etc
	for i, obj := range m.pageData {
		selected := i == m.pageIndex
		checked := m.checked[i]
		if selected {
			out.WriteString(cursor)
			out.WriteString(" ")
		} else {
			out.WriteString(strings.Repeat(" ", runewidth.StringWidth(cursor)+1))
		}

		if checked {
			out.WriteString("[x] ")
		} else {
			out.WriteString("[ ] ")
		}

		out.WriteString(obj.Render(selected, checked))
		out.WriteString("\n")
	}

	return out.String()
}

// Update method responds to various events and modifies the data model
// according to the corresponding events
func (m Model[I]) Update(msg tea.Msg) (Model[I], tea.Cmd) {
	if !m.init {
		m.initData()
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "down":
			m.moveDown()
		case "up":
			m.moveUp()
		case "right", "pgdown", "l", "k":
			m.nextPage()
		case "left", "pgup", "h", "j":
			m.prePage()
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			num, _ := strconv.Atoi(msg.String())
			idx := num - 1
			m.forward(idx)

		case "x", " ":
			m.toggle()
		}
	}
	return m, nil
}

func (m *Model[I]) toggle() {
	idx := m.pageIndex
	if idx >= 0 && idx < len(m.pageData) {
		m.checked[idx] = !m.checked[idx]
	}
}

// moveDown executes the downward movement of the cursor,
// while adjusting the internal index and refreshing the data area
func (m *Model[I]) moveDown() {
	// the page index has not reached the maximum value, and the page
	// data area does not need to be updated
	if m.pageIndex < m.pageMaxIndex {
		m.pageIndex++
		// check whether the global index reaches the maximum value before sliding
		if m.index < m.maxIndex {
			m.index++
		}
		return
	}

	// the page index reaches the maximum value, slide the page data area window,
	// the page index maintains the maximum value
	if m.pageIndex == m.pageMaxIndex {
		// check whether the global index reaches the maximum value before sliding
		if m.index < m.maxIndex {
			// global index increment
			m.index++
			// window slide down one data
			m.pageData = m.Data[m.index+1-m.PerPage : m.index+1]
			return
		}
	}
}

// moveUp performs an upward movement of the cursor,
// while adjusting the internal index and refreshing the data area
func (m *Model[I]) moveUp() {
	// the page index has not reached the minimum value, and the page
	// data area does not need to be updated
	if m.pageIndex > 0 {
		m.pageIndex--
		// check whether the global index reaches the minimum before sliding
		if m.index > 0 {
			m.index--
		}
		return
	}

	// the page index reaches the minimum value, slide the page data window,
	// and the page index maintains the minimum value
	if m.pageIndex == 0 {
		// check whether the global index reaches the minimum before sliding
		if m.index > 0 {
			// window slide up one data
			m.pageData = m.Data[m.index-1 : m.index-1+m.PerPage]
			// global index decrement
			m.index--
			return
		}
	}
}

// nextPage triggers the page-down action, and does not change
// the real-time page index(pageIndex)
func (m *Model[I]) nextPage() {
	// Get the start and end position of the page data area slice: m.Data[start:end]
	//
	// note: the slice is closed left and opened right: `[start,end)`
	//       assuming that the global data area has unlimited length,
	//       end should always be the actual page `length+1`,
	//       the maximum value of end should be equal to `len(m.Data)`
	//       under limited length
	pageStart, pageEnd := m.pageIndexInfo()
	// there are two cases when `end` does not reach the maximum value
	if pageEnd < len(m.Data) {
		// the `end` value is at least one page length away from the global maximum index
		if len(m.Data)-pageEnd >= m.PerPage {
			// slide back one page in the page data area
			m.pageData = m.Data[pageStart+m.PerPage : pageEnd+m.PerPage]
			// Global real-time index increases by one page length
			m.index += m.PerPage
		} else { // `end` is less than a page length from the global maximum index
			// slide the page data area directly to the end
			m.pageData = m.Data[len(m.Data)-m.PerPage : len(m.Data)]
			// `sliding distance` = `position after sliding` - `position before sliding`
			// the global real-time index should also synchronize the same sliding distance
			m.index += len(m.Data) - pageEnd
		}
	}
}

// prePage triggers the page-up action, and does not change
// the real-time page index(pageIndex)
func (m *Model[I]) prePage() {
	// Get the start and end position of the page data area slice: m.Data[start:end]
	//
	// note: the slice is closed left and opened right: `[start,end)`
	//       assuming that the global data area has unlimited length,
	//       end should always be the actual page `length+1`,
	//       the maximum value of end should be equal to `len(m.Data)`
	//       under limited length
	pageStart, pageEnd := m.pageIndexInfo()
	// there are two cases when `start` does not reach the minimum value
	if pageStart > 0 {
		// `start` is at least one page length from the minimum
		if pageStart >= m.PerPage {
			// slide the page data area forward one page
			m.pageData = m.Data[pageStart-m.PerPage : pageEnd-m.PerPage]
			// Global real-time index reduces the length of one page
			m.index -= m.PerPage
		} else { // `start` to the minimum value less than one page length
			// slide the page data area directly to the start
			m.pageData = m.Data[:m.PerPage]
			// `sliding distance` = `position before sliding` - `minimum value(0)`
			// the global real-time index should also synchronize the same sliding distance
			m.index -= pageStart - 0
		}
	}
}

// forward triggers a fast jump action, if the pageIndex
// is invalid, keep it as it is
func (m *Model[I]) forward(pageIndex int) {
	// pageIndex has exceeded the maximum index of the page, ignore
	if pageIndex > m.pageMaxIndex {
		return
	}

	// calculate the distance moved to pageIndex
	l := pageIndex - m.pageIndex
	// update the global real time index
	m.index += l
	// update the page real time index
	m.pageIndex = pageIndex

}

// initData initialize the data model, set the default value and
// fix the wrong parameter settings during initialization
func (m *Model[I]) initData() {
	if m.PerPage > len(m.Data) || m.PerPage < 1 {
		m.PerPage = len(m.Data)
		m.pageData = m.Data
	} else {
		m.pageData = m.Data[:m.PerPage]
	}

	m.pageIndex = 0
	m.pageMaxIndex = m.PerPage - 1
	m.index = 0
	m.maxIndex = len(m.Data) - 1
	m.checked = make([]bool, len(m.Data))
	copy(m.checked, m.Initial)
	m.init = true
}

// pageIndexInfo return the start and end positions of the slice of the
// page data area corresponding to the global data area
func (m *Model[I]) pageIndexInfo() (start, end int) {
	// `Global real-time index` - `page real-time index` = `start index of page data area`
	start = m.index - m.pageIndex
	// `Page data area start index` + `single page size` = `page data area end index`
	end = start + m.PerPage
	return
}
