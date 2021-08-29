package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"fx_alert/pkg/db"
	"fx_alert/pkg/telegram"
)

type CommandType string

const (
	AddLevelValue    CommandType = "/ladd"
	DeleteLevelValue CommandType = "/ldel"
	ListLevelValues  CommandType = "/lls"
	AddDeltaValue    CommandType = "/dadd"
	DeleteDeltaValue CommandType = "/ddel"
	ListDeltaValues  CommandType = "/dls"
	Help             CommandType = "/help"

	DeltaAllSymbols = "*"
)

func CommandFromString(txt string) (CommandType, error) {
	txt = strings.TrimSpace(strings.ToLower(txt))
	switch {
	case strings.HasPrefix(txt, string(AddLevelValue)+" "):
		return AddLevelValue, nil
	case strings.HasPrefix(txt, string(DeleteLevelValue)+" ") || (txt == string(DeleteLevelValue)):
		return DeleteLevelValue, nil
	case txt == string(ListLevelValues):
		return ListLevelValues, nil
	case strings.HasPrefix(txt, string(AddDeltaValue)+" "):
		return AddDeltaValue, nil
	case strings.HasPrefix(txt, string(DeleteDeltaValue)+" ") || (txt == string(DeleteDeltaValue)):
		return DeleteDeltaValue, nil
	case txt == string(ListDeltaValues):
		return ListDeltaValues, nil
	case txt == string(Help):
		return Help, nil
	}

	return "", errors.New("Unsupported command")
}

type CommandValue struct {
	Command CommandType
	Level   *db.LevelValue
	Delta   *db.DeltaValue
}

func Parse(msg string) (*CommandValue, error) {
	msg = strings.TrimSpace(strings.ToLower(msg))
	cmdT, err := CommandFromString(msg)
	if err != nil {
		return nil, fmt.Errorf("Can't parse command: %w", err)
	}

	if cmdT == ListLevelValues {
		return &CommandValue{Command: ListLevelValues}, nil
	}
	if cmdT == ListDeltaValues {
		return &CommandValue{Command: ListDeltaValues}, nil
	}
	if cmdT == Help {
		return &CommandValue{Command: Help}, nil
	}

	if cmdT == AddLevelValue {
		v, err := parseLevelValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Level:   v,
		}

		return cv, nil
	}
	if cmdT == AddDeltaValue {
		v, err := parseDeltaValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Delta:   v,
		}

		return cv, nil
	}

	if cmdT == DeleteLevelValue {
		if msg == string(DeleteLevelValue) {
			return &CommandValue{Command: DeleteLevelValue}, nil
		}
		v, err := parseLevelValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Level:   v,
		}

		return cv, nil
	}
	if cmdT == DeleteDeltaValue {
		if msg == string(DeleteDeltaValue) {
			return &CommandValue{Command: DeleteDeltaValue}, nil
		}
		v, err := parseDeltaValue(msg)
		if err != nil {
			return nil, err
		}
		cv := &CommandValue{
			Command: cmdT,
			Delta:   v,
		}

		return cv, nil
	}

	return nil, errors.New("Unsupported command")
}

func parseLevelValue(msg string) (*db.LevelValue, error) {
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

	return &db.LevelValue{
		Key:   k,
		Value: v,
		Type:  vt,
	}, nil
}

func parseDeltaValue(msg string) (*db.DeltaValue, error) {
	parts := strings.Split(msg, " ")
	if len(parts) != 4 {
		return nil, errors.New("Unsupported command format")
	}
	k := strings.ToUpper(parts[1])
	rawVal := strings.ReplaceAll(parts[2], ",", ".")
	v, err := strconv.ParseFloat(rawVal, 64)
	if err != nil {
		return nil, fmt.Errorf("Can't parse value: %q. %w", rawVal, err)
	}
	rawD := strings.ReplaceAll(parts[3], ",", ".")
	d, err := strconv.ParseFloat(rawD, 64)
	if err != nil {
		return nil, fmt.Errorf("Can't parse delta: %q. %w", rawVal, err)
	}

	return &db.DeltaValue{
		Key:   k,
		Value: v,
		Delta: d,
	}, nil
}

func HelpAnswer() *telegram.Answer {
	answer := fmt.Sprintf(
		`
Add level: %s EURUSD %s 1.2550
Delete level: %s EURUSD %s 1.2550
Keyboard delete level: %s
List levels: %s
Add delta: %s EURUSD 1.2550 500
Delete delta: %s EURUSD 1.2550 500
Keyboard delete delta: %s
List deltas: %s
Help: %s
`,
		AddLevelValue,
		db.AboveCurrent,
		DeleteLevelValue,
		db.BelowCurrent,
		DeleteLevelValue,
		ListLevelValues,
		AddDeltaValue,
		DeleteDeltaValue,
		DeleteDeltaValue,
		ListDeltaValues,
		Help,
	)

	return &telegram.Answer{Text: answer}
}
