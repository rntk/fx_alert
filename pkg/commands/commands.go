package commands

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"fx_alert/pkg/db"
	"fx_alert/pkg/quoter"
	"fx_alert/pkg/telegram"
)

type CommandType string

var clearWhiteSpace = regexp.MustCompile(`\s+`)

const (
	AddValue    CommandType = "/add"
	DeleteValue CommandType = "/del"
	ListValues  CommandType = "/ls"
	DeltaValue  CommandType = "/delta"
	Help        CommandType = "/help"

	NoValue = -1

	AnySymbol = "*"
)

func CommandFromString(txt string) (CommandType, error) {
	txt = strings.TrimSpace(strings.ToLower(txt))
	switch {
	case strings.HasPrefix(txt, string(AddValue)+" "):
		return AddValue, nil
	case strings.HasPrefix(txt, string(DeleteValue)+" ") || (txt == string(DeleteValue)):
		return DeleteValue, nil
	case strings.HasPrefix(txt, string(ListValues)+" ") || (txt == string(ListValues)):
		return ListValues, nil
	case strings.HasPrefix(txt, string(DeltaValue)+" "):
		return DeltaValue, nil
	case txt == string(Help):
		return Help, nil
	}

	return "", errors.New("Unsupported command")
}

type CommandValue struct {
	Command CommandType
	Value   *db.Value
}

func Parse(msg string) (*CommandValue, error) {
	msg = strings.TrimSpace(strings.ToLower(msg))
	msg = clearWhiteSpace.ReplaceAllString(msg, " ")
	cmdT, err := CommandFromString(msg)
	if err != nil {
		return nil, fmt.Errorf("Can't parse command: %w", err)
	}

	if cmdT == ListValues {
		if msg == string(ListValues) {
			return &CommandValue{Command: ListValues}, nil
		}
		cv := &CommandValue{
			Command: cmdT,
			Value: &db.Value{
				Key: strings.TrimSpace(strings.TrimPrefix(msg, string(ListValues))),
			},
		}
		return cv, nil
	}
	if cmdT == Help {
		return &CommandValue{Command: Help}, nil
	}

	if cmdT == AddValue {
		v, err := parseValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Value:   v,
		}

		return cv, nil
	}

	if cmdT == DeleteValue {
		if msg == string(DeleteValue) {
			return &CommandValue{Command: DeleteValue}, nil
		}
		if strings.Count(msg, " ") == 1 {
			cv := &CommandValue{
				Command: cmdT,
				Value: &db.Value{
					Key:   strings.TrimSpace(strings.TrimPrefix(msg, string(DeleteValue))),
					Value: NoValue,
				},
			}
			return cv, nil
		}
		v, err := parseValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Value:   v,
		}

		return cv, nil
	}

	if cmdT == DeltaValue {
		v, err := parseDeltaValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Value:   v,
		}

		return cv, nil
	}

	return nil, errors.New("Unsupported command")
}

func parseValue(msg string) (*db.Value, error) {
	parts := strings.Split(msg, " ")
	if len(parts) != 4 {
		return nil, errors.New("Unsupported command format")
	}
	k := strings.ToUpper(parts[1])
	vt, err := db.ValueTypeFromString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("Can't parse value type: %w", err)
	}
	rawVal := strings.ReplaceAll(parts[3], ",", ".")
	v, err := strconv.ParseFloat(rawVal, 64)
	if err != nil {
		return nil, fmt.Errorf("Can't parse value: %q. %w", rawVal, err)
	}

	return &db.Value{
		Key:       k,
		Value:     v,
		Type:      vt,
		Precision: quoter.GetPrecision(k),
	}, nil
}

func parseDeltaValue(msg string) (*db.Value, error) {
	parts := strings.Split(msg, " ")
	ln := len(parts)
	if (ln != 2) && (ln != 3) {
		return nil, errors.New("Unsupported command format")
	}
	var rawVal string
	var k string
	if ln == 2 {
		rawVal = parts[1]
	} else {
		rawVal = parts[2]
		k = parts[1]
	}
	rawVal = strings.ReplaceAll(rawVal, ",", ".")
	v, err := strconv.ParseInt(rawVal, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Can't parse delta value: %q. %w", rawVal, err)
	}
	if v <= 0 {
		return nil, errors.New("Delta value must be > 0")
	}

	return &db.Value{Key: k, Value: float64(v)}, nil
}

func HelpAnswer() *telegram.Answer {
	answer := fmt.Sprintf(
		`
Add: %[3]s EURUSD %[1]s 1.2550

Delete: %[4]s EURUSD %[2]s 1.2550
Delete: %[4]s EURUSD
Delete: %[4]s EUR
Delete: %[4]s *

Keyboard delete: %[4]s

List: %[5]s
List: %[5]s USD

Delta: %[6]s USDJPY 500
Delta: %[6]s USD 500
Delta: %[6]s 500

Help: %[7]s
`,
		db.AboveCurrent,
		db.BelowCurrent,
		AddValue,
		DeleteValue,
		ListValues,
		DeltaValue,
		Help,
	)

	return &telegram.Answer{Text: answer}
}
