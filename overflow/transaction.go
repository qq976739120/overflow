package overflow

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/enescakir/emoji"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-cli/pkg/flowkit"
	"github.com/onflow/flow-cli/pkg/flowkit/output"
	"github.com/onflow/flow-go-sdk"
	"github.com/pkg/errors"
)

type OverFlowEventFilter map[string][]string

type OverflowEvents map[string][]OverflowEvent

type OverflowEvent map[string]interface{}

func (o *OverflowState) SimpleTxArgs(filename string, signer string, args *FlowArgumentsBuilder) {
	o.TransactionFromFile(filename).SignProposeAndPayAs(signer).Args(args).RunPrintEventsFull()
}

// TransactionFromFile will start a flow transaction builder
func (o *OverflowState) TransactionFromFile(filename string) FlowInteractionBuilder {
	return FlowInteractionBuilder{
		Overflow:       o,
		FileName:       filename,
		MainSigner:     nil,
		Arguments:      []cadence.Value{},
		PayloadSigners: []*flowkit.Account{},
		GasLimit:       uint64(o.Gas),
		BasePath:       fmt.Sprintf("%s/transactions", o.BasePath),
	}
}

// Transaction will start a flow transaction builder using the inline transaction
func (o *OverflowState) Transaction(content string) FlowInteractionBuilder {
	return FlowInteractionBuilder{
		Overflow:       o,
		FileName:       "inline",
		Content:        content,
		MainSigner:     nil,
		Arguments:      []cadence.Value{},
		PayloadSigners: []*flowkit.Account{},
		GasLimit:       uint64(o.Gas),
		BasePath:       fmt.Sprintf("%s/transactions", o.BasePath),
	}
}

// Deprecated: Use ArgM
func (t FlowInteractionBuilder) NamedArguments(args map[string]string) FlowInteractionBuilder {

	codeFileName := fmt.Sprintf("%s/%s.cdc", t.BasePath, t.FileName)
	code, err := t.getContractCode(codeFileName)
	if err != nil {
		fmt.Println(err.Error())
		t.Error = err
	}
	parseArgs, err := t.Overflow.ParseArgumentsWithoutType(t.FileName, code, args)
	if err != nil {
		t.Error = err
	}
	t.Arguments = parseArgs
	return t
}

// Specify arguments to send to transaction using a raw list of values
// Deprecated: Use Args
func (t FlowInteractionBuilder) ArgsV(args []cadence.Value) FlowInteractionBuilder {
	t.Arguments = args
	return t
}

// Specify arguments to send to transaction using a builder you send in
// Deprecated: Use Arg
func (t FlowInteractionBuilder) Args(args *FlowArgumentsBuilder) FlowInteractionBuilder {
	t.Arguments = args.Build()
	return t
}

// Specify arguments to send to transaction using a function that takes a builder where you call the builder
// Deprecated: Use Arg
func (t FlowInteractionBuilder) ArgsFn(fn func(*FlowArgumentsBuilder)) FlowInteractionBuilder {
	args := t.Overflow.Arguments()
	fn(args)
	t.Arguments = args.Build()
	return t
}

func (t FlowInteractionBuilder) TransactionPath(path string) FlowInteractionBuilder {
	t.BasePath = path
	return t
}

// Gas sets the gas limit for this transaction
func (t FlowInteractionBuilder) Gas(limit uint64) FlowInteractionBuilder {
	t.GasLimit = limit
	return t
}

// SignProposeAndPayAs set the payer, proposer and envelope signer
func (t FlowInteractionBuilder) SignProposeAndPayAs(signer string) FlowInteractionBuilder {
	account, err := t.Overflow.AccountE(signer)
	if err != nil {
		t.Error = err
		return t
	}
	t.Proposer = account
	t.MainSigner = account
	return t
}

// SignProposeAndPayAsService set the payer, proposer and envelope signer
func (t FlowInteractionBuilder) SignProposeAndPayAsService() FlowInteractionBuilder {
	key := t.Overflow.ServiceAccountName()
	//swallow error as you cannot start a overflow without a valid sa
	account, _ := t.Overflow.State.Accounts().ByName(key)
	t.MainSigner = account
	t.Proposer = account
	return t
}

// PayloadSigner set a signer for the payload
func (t FlowInteractionBuilder) PayloadSigner(value string) FlowInteractionBuilder {
	signer, err := t.Overflow.AccountE(value)
	if err != nil {
		t.Error = err
		return t
	}
	t.PayloadSigners = append(t.PayloadSigners, signer)
	return t
}

// RunPrintEventsFull will run a transaction and print all events
//Deprecated use Send().PrintEvents()
func (t FlowInteractionBuilder) RunPrintEventsFull() {
	PrintEvents(t.Run(), map[string][]string{})
}

// RunPrintEvents will run a transaction and print all events ignoring some fields
//Deprecated use Send().PrintEventsFiltered()
func (t FlowInteractionBuilder) RunPrintEvents(ignoreFields map[string][]string) {
	PrintEvents(t.Run(), ignoreFields)
}

// Run run the transaction
// deprecated: use Send and get entire result
func (t FlowInteractionBuilder) Run() []flow.Event {
	result := t.Send()
	if result.Err != nil {
		t.Overflow.Logger.Error(fmt.Sprintf("%v Error executing script: %s output %v", emoji.PileOfPoo, t.FileName, result.Err))
		panic(result.Err)
	}
	return result.RawEvents
}

func (t FlowInteractionBuilder) RunGetIdFromEventPrintAll(eventName string, fieldName string) uint64 {
	result := t.Send()
	if result.Err != nil {
		panic(result.Err)
	}

	PrintEvents(result.RawEvents, map[string][]string{})

	return result.GetIdFromEvent(eventName, fieldName)
}

// Deprecated, use Send().GetIdFromEvent
func (t FlowInteractionBuilder) RunGetIdFromEvent(eventName string, fieldName string) uint64 {

	result := t.Send()
	if result.Err != nil {
		panic(result.Err)
	}
	return result.GetIdFromEvent(eventName, fieldName)
}

func (t FlowInteractionBuilder) RunGetIds(eventName string, fieldName string) ([]uint64, error) {

	result := t.Send()
	if result.Err != nil {
		return nil, result.Err
	}
	return result.GetIdsFromEvent(eventName, fieldName), nil
}

/// Deprecated: use Send().GetEventsWithName
func (t FlowInteractionBuilder) RunGetEventsWithNameOrError(eventName string) ([]FormatedEvent, error) {

	result := t.Send()
	if result.Err != nil {
		return nil, result.Err
	}
	var events []FormatedEvent
	for _, event := range result.RawEvents {
		ev := ParseEvent(event, uint64(0), time.Unix(0, 0), []string{})
		if ev.Name == eventName {
			events = append(events, *ev)
		}
	}
	return events, nil

}

// Deprecated: Use Send().GetEventsWithName()
func (t FlowInteractionBuilder) RunGetEventsWithName(eventName string) []FormatedEvent {

	result := t.Send()
	if result.Err != nil {
		panic(result.Err)
	}
	var events []FormatedEvent
	for _, event := range result.RawEvents {
		ev := ParseEvent(event, uint64(0), time.Unix(0, 0), []string{})
		if ev.Name == eventName {
			events = append(events, *ev)
		}
	}
	return events
}

// RunE runs returns events and error
// Deprecated: use Send()
func (t FlowInteractionBuilder) RunE() ([]flow.Event, error) {
	result := t.Send()
	return result.RawEvents, result.Err
}

// The new main way of running an overflow transaction
func (t FlowInteractionBuilder) Send() *OverflowResult {
	result := &OverflowResult{}
	if t.Error != nil {
		result.Err = t.Error
		return result
	}

	if t.Proposer == nil {
		result.Err = fmt.Errorf("%v You need to set the main signer", emoji.PileOfPoo)
		return result
	}

	codeFileName := fmt.Sprintf("%s/%s.cdc", t.BasePath, t.FileName)

	if len(t.TransactionCode) == 0 {
		code, err := t.getContractCode(codeFileName)
		if err != nil {
			result.Err = err
			return result
		}
		t.TransactionCode = code
	}

	t.Overflow.Log.Reset()
	t.Overflow.EmulatorLog.Reset()
	// we append the mainSigners at the end here so that it signs last
	signers := t.PayloadSigners
	if t.MainSigner != nil {
		signers = append(signers, t.MainSigner)
	}

	var authorizers []flow.Address
	for _, signer := range signers {
		authorizers = append(authorizers, signer.Address())
	}
	if t.MainSigner == nil {
		signers = append(signers, t.Proposer)
	}

	tx, err := t.Overflow.Services.Transactions.Build(
		t.Proposer.Address(),
		authorizers,
		t.Proposer.Address(),
		t.Proposer.Key().Index(),
		t.TransactionCode,
		codeFileName,
		t.GasLimit,
		t.Arguments,
		t.Overflow.Network,
		true,
	)
	if err != nil {
		result.Err = err
		return result
	}

	for _, signer := range signers {
		err = tx.SetSigner(signer)
		if err != nil {
			result.Err = err
			return result
		}

		tx, err = tx.Sign()
		if err != nil {
			result.Err = err
			return result
		}
	}
	txId := tx.FlowTransaction().ID()
	result.Id = txId

	txBytes := []byte(fmt.Sprintf("%x", tx.FlowTransaction().Encode()))
	ftx, res, err := t.Overflow.Services.Transactions.SendSigned(txBytes, true)
	result.Transaction = ftx

	if err != nil {
		result.Err = err
		return result
	}

	var logMessage []LogrusMessage
	dec := json.NewDecoder(t.Overflow.Log)
	for {
		var doc LogrusMessage

		err := dec.Decode(&doc)
		if err == io.EOF {
			// all done
			break
		}
		if err != nil {
			result.Err = err
			return result
		}

		logMessage = append(logMessage, doc)
	}

	result.Meter = &Meter{}
	var meter Meter
	scanner := bufio.NewScanner(t.Overflow.EmulatorLog)
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Contains(txt, "transaction execution data") {
			err = json.Unmarshal([]byte(txt), &meter)
			if err == nil {
				result.Meter = &meter
			}
		}
	}
	messages := []string{}
	for _, msg := range logMessage {
		if msg.ComputationUsed != 0 {
			result.ComputationUsed = msg.ComputationUsed
		}
		messages = append(messages, msg.Msg)
	}
	result.RawLog = logMessage

	result.EmulatorLog = messages

	result.RawEvents = res.Events

	overflowEvents := OverflowEvents{}
	for _, event := range result.RawEvents {

		var fieldNames []string

		for _, eventTypeFields := range event.Value.EventType.Fields {
			fieldNames = append(fieldNames, eventTypeFields.Identifier)
		}

		finalFields := map[string]interface{}{}

		for id, field := range event.Value.Fields {
			name := fieldNames[id]
			finalFields[name] = CadenceValueToInterfaceCompact(field)
		}

		if strings.HasSuffix(event.Type, "FlowFees.FeesDeducted") {
			result.Fee = finalFields
		}

		events, ok := overflowEvents[event.Type]
		if !ok {
			events = []OverflowEvent{}
		}
		events = append(events, finalFields)
		overflowEvents[event.Type] = events
	}

	if !t.IgnoreGlobalEventFilters {

		fee := result.Fee["amount"]
		if t.Overflow.FilterOutFeeEvents && fee != nil {
			overflowEvents = overflowEvents.FilterFees(fee.(float64))
		}

		if t.Overflow.FilterOutEmptyWithDrawDepositEvents {
			overflowEvents = overflowEvents.FilterTempWithdrawDeposit()
		}

		if len(t.Overflow.GlobalEventFilter) != 0 {
			overflowEvents = overflowEvents.FilterEvents(t.Overflow.GlobalEventFilter)
		}
	}

	if len(t.EventFilter) != 0 {
		overflowEvents = overflowEvents.FilterEvents(t.EventFilter)
	}

	result.Events = overflowEvents

	result.Logger = t.Overflow.Logger

	result.Name = t.FileName
	t.Overflow.Log.Reset()
	t.Overflow.EmulatorLog.Reset()
	result.Err = res.Error
	return result
}

func (t FlowInteractionBuilder) getContractCode(codeFileName string) ([]byte, error) {
	code := []byte(t.Content)
	var err error
	if t.Content == "" {
		code, err = t.Overflow.State.ReaderWriter().ReadFile(codeFileName)
		if err != nil {
			return nil, fmt.Errorf("%v Could not read transaction file from path=%s", emoji.PileOfPoo, codeFileName)
		}
	}
	return code, nil
}

// FlowInteractionBuilder used to create a builder pattern for a transaction
type FlowInteractionBuilder struct {
	Overflow       *OverflowState
	FileName       string
	Content        string
	Arguments      []cadence.Value
	MainSigner     *flowkit.Account
	PayloadSigners []*flowkit.Account
	GasLimit       uint64
	BasePath       string
	Error          error

	//these are used for v3, but can still be here for v2
	TransactionCode []byte
	NamedArgs       map[string]interface{}
	Proposer        *flowkit.Account

	EventFilter              OverFlowEventFilter
	IgnoreGlobalEventFilters bool
}

type OverflowScriptResult struct {
	Err    error
	Result cadence.Value
	Input  *FlowInteractionBuilder
	Log    []LogrusMessage
}

func (osr *OverflowScriptResult) GetAsJson() string {
	if osr.Err != nil {
		panic(fmt.Sprintf("%v Error executing script: %s output %v", emoji.PileOfPoo, osr.Input.FileName, osr.Err))
	}
	return CadenceValueToJsonStringCompact(osr.Result)
}

func (osr *OverflowScriptResult) GetAsInterface() interface{} {
	if osr.Err != nil {
		panic(fmt.Sprintf("%v Error executing script: %s output %v", emoji.PileOfPoo, osr.Input.FileName, osr.Err))
	}
	return CadenceValueToInterfaceCompact(osr.Result)
}

func (osr *OverflowScriptResult) Print() {
	json := osr.GetAsJson()
	osr.Input.Overflow.Logger.Info(fmt.Sprintf("%v Script %s run from result: %v\n", emoji.Star, osr.Input.FileName, json))
}

func (osr *OverflowScriptResult) MarhalAs(value interface{}) error {
	if osr.Err != nil {
		return osr.Err
	}
	jsonResult := CadenceValueToJsonStringCompact(osr.Result)
	err := json.Unmarshal([]byte(jsonResult), &value)
	return err
}

type OverflowResult struct {
	//The error if any
	Err error

	//The id of the transaction
	Id flow.Identifier

	//If running on an emulator
	//the meter that contains useful debug information on memory and interactions
	Meter *Meter
	//The Raw log from the emulator
	RawLog []LogrusMessage
	// The log from the emulator
	EmulatorLog []string

	//The computation used
	ComputationUsed int

	//The raw unfiltered events
	RawEvents []flow.Event

	//Events that are filtered and parsed into a terse format
	Events OverflowEvents

	//The underlying transaction if we need to look into that
	Transaction *flow.Transaction

	//The fee event if any
	Fee map[string]interface{}

	//The logger to log output to
	Logger output.Logger

	//The name of the Transaction
	Name string
}

func (overflowEvents OverflowEvents) FilterTempWithdrawDeposit() OverflowEvents {
	filteredEvents := overflowEvents
	for name, events := range overflowEvents {
		if strings.HasSuffix(name, "TokensWithdrawn") {

			withDrawnEvents := []OverflowEvent{}
			for _, value := range events {
				if value["from"] != nil {
					withDrawnEvents = append(withDrawnEvents, value)
				}
			}
			if len(withDrawnEvents) != 0 {
				filteredEvents[name] = withDrawnEvents
			} else {
				delete(filteredEvents, name)
			}
		}

		if strings.HasSuffix(name, "TokensDeposited") {
			despoitEvents := []OverflowEvent{}
			for _, value := range events {
				if value["to"] != nil {
					despoitEvents = append(despoitEvents, value)
				}
			}
			if len(despoitEvents) != 0 {
				filteredEvents[name] = despoitEvents
			} else {
				delete(filteredEvents, name)
			}
		}
	}
	return filteredEvents
}

func (overflowEvents OverflowEvents) FilterFees(fee float64) OverflowEvents {

	filteredEvents := overflowEvents
	for name, events := range overflowEvents {
		if strings.HasSuffix(name, "FlowFees.FeesDeducted") {
			delete(filteredEvents, name)
		}

		if strings.HasSuffix(name, "FlowToken.TokensWithdrawn") {

			withDrawnEvents := []OverflowEvent{}
			for _, value := range events {
				if value["amount"].(float64) != fee {
					withDrawnEvents = append(withDrawnEvents, value)
				}
			}
			if len(withDrawnEvents) != 0 {
				filteredEvents[name] = withDrawnEvents
			} else {
				delete(filteredEvents, name)
			}
		}

		if strings.HasSuffix(name, "FlowToken.TokensDeposited") {
			despoitEvents := []OverflowEvent{}
			for _, value := range events {
				if value["amount"].(float64) != fee {
					despoitEvents = append(despoitEvents, value)
				}
			}
			if len(despoitEvents) != 0 {
				filteredEvents[name] = despoitEvents
			} else {
				delete(filteredEvents, name)
			}

		}
	}
	return filteredEvents
}
func (overflowEvents OverflowEvents) FilterEvents(ignoreFields OverFlowEventFilter) OverflowEvents {
	filteredEvents := OverflowEvents{}
	for name, events := range overflowEvents {

		//find if we should ignore fields
		ignoreFieldNames := []string{}
		for ignoreEvent, fields := range ignoreFields {
			if strings.HasSuffix(name, ignoreEvent) {
				ignoreFieldNames = fields
			}
		}

		eventList := []OverflowEvent{}
		for _, ev := range events {
			event := OverflowEvent{}
			for key, value := range ev {
				valid := true
				for _, ig := range ignoreFieldNames {
					if strings.HasSuffix(key, ig) {
						valid = false
					}
				}
				if valid {
					event[key] = value
				}
			}
			if len(event) != 0 {
				eventList = append(eventList, event)
			}
		}
		if len(eventList) != 0 {
			filteredEvents[name] = eventList
		}
	}
	return filteredEvents
}

type PrinterOption func(*PrintOptions)
type PrintOptions struct {
	Events      bool
	EventFilter OverFlowEventFilter
	Meter       bool
	EmulatorLog bool
}

func WithMeter() func(opt *PrintOptions) {
	return func(opt *PrintOptions) {
		opt.Meter = true
	}
}

func WithEmulatorLog() func(opt *PrintOptions) {
	return func(opt *PrintOptions) {
		opt.EmulatorLog = true
	}
}

func WithEventFilter(filter OverFlowEventFilter) func(opt *PrintOptions) {
	return func(opt *PrintOptions) {
		opt.EventFilter = filter
	}
}

func WithoutEvents() func(opt *PrintOptions) {
	return func(opt *PrintOptions) {
		opt.Events = false
	}
}

func (o OverflowResult) Print(opts ...PrinterOption) {

	printOpts := &PrintOptions{
		Events:      true,
		EventFilter: OverFlowEventFilter{},
		Meter:       false,
		EmulatorLog: false,
	}

	for _, opt := range opts {
		opt(printOpts)
	}

	if o.Err != nil {
		o.Logger.Error(fmt.Sprintf("%v Error executing transaction: %s", emoji.PileOfPoo, o.Name))
		panic(o.Err)
	}

	messages := []string{}

	if o.ComputationUsed != 0 {
		messages = append(messages, fmt.Sprintf("%d%v", o.ComputationUsed, emoji.HighVoltage))
	}
	nameMessage := fmt.Sprintf("Tx %s", o.Name)
	if o.Name == "inline" {
		nameMessage = "Inline TX"
	}
	messages = append(messages, nameMessage)

	if len(o.Fee) != 0 {
		messages = append(messages, fmt.Sprintf("%v:%f (%f/%f)", emoji.MoneyBag, o.Fee["amount"].(float64), o.Fee["inclusionEffort"].(float64), o.Fee["exclusionEffort"].(float64)))
	}
	messages = append(messages, fmt.Sprintf("id:%s", o.Id.String()))

	o.Logger.Info(fmt.Sprintf("%v %s", emoji.OkHand, strings.Join(messages, " ")))

	if printOpts.Events {
		events := o.Events
		if len(printOpts.EventFilter) != 0 {
			events = events.FilterEvents(printOpts.EventFilter)
		}
		if len(events) != 0 {
			o.Logger.Info("=== Events ===")
			for name, eventList := range events {
				for _, event := range eventList {
					o.Logger.Info(name)
					for key, value := range event {
						o.Logger.Info(fmt.Sprintf("   %s:%v", key, value))
					}
				}
			}
		}
	}

	if printOpts.EmulatorLog && len(o.RawLog) > 0 {
		o.Logger.Info("=== LOG ===")
		for _, msg := range o.RawLog {
			o.Logger.Info(msg.Msg)
		}
	}

	if printOpts.Meter && o.Meter != nil {
		fmt.Println("=== METER ===")
		fmt.Println("LedgerInteractionUsed:", o.Meter.LedgerInteractionUsed)
		if o.Meter.MemoryUsed != 0 {
			fmt.Println("Memory:", o.Meter.MemoryUsed)
			memories := strings.ReplaceAll(strings.Trim(fmt.Sprintf("%+v", o.Meter.MemoryIntensities), "map[]"), " ", "\n  ")

			fmt.Println("Memory Intensities")
			fmt.Println(" ", memories)
		}
		fmt.Println("Computation:", o.Meter.ComputationUsed)
		intensities := strings.ReplaceAll(strings.Trim(fmt.Sprintf("%+v", o.Meter.ComputationIntensities), "map[]"), " ", "\n  ")

		fmt.Println("Computation Intensities:")
		fmt.Println(" ", intensities)
	}
}

func (o OverflowResult) GetIdFromEvent(eventName string, fieldName string) uint64 {
	for name, event := range o.Events {
		if strings.HasSuffix(name, eventName) {
			return event[0][fieldName].(uint64)
		}
	}
	panic(fmt.Sprintf("Could not find id field %s in event with suffix %s", fieldName, eventName))
}

func (o OverflowResult) GetIdsFromEvent(eventName string, fieldName string) []uint64 {
	var ids []uint64
	for name, events := range o.Events {
		if strings.HasSuffix(name, eventName) {
			for _, event := range events {
				ids = append(ids, event[fieldName].(uint64))
			}
		}
	}
	return ids
}

func (o OverflowResult) GetEventsWithName(eventName string) []OverflowEvent {
	for name, event := range o.Events {
		if strings.HasSuffix(name, eventName) {
			return event
		}
	}
	panic(fmt.Sprintf("Could not events with suffix %s", eventName))
}

// v3

//A function to customize the transaction builder
type TransactionOption func(*FlowInteractionBuilder)

type TransactionFunction func(filename string, opts ...TransactionOption) *OverflowResult
type TransactionOptsFunction func(opts ...TransactionOption) *OverflowResult

type ScriptFunction func(filename string, opts ...TransactionOption) *OverflowScriptResult
type ScriptOptsFunction func(opts ...TransactionOption) *OverflowScriptResult

func (o *OverflowState) ScriptFN(outerOpts ...TransactionOption) ScriptFunction {

	return func(filename string, opts ...TransactionOption) *OverflowScriptResult {

		for _, opt := range opts {
			outerOpts = append(outerOpts, opt)
		}
		return o.Script(filename, outerOpts...)
	}
}

func (o *OverflowState) TxFN(outerOpts ...TransactionOption) TransactionFunction {

	return func(filename string, opts ...TransactionOption) *OverflowResult {

		for _, opt := range opts {
			outerOpts = append(outerOpts, opt)
		}
		return o.Tx(filename, outerOpts...)

	}
}

func (o *OverflowState) ScriptFileNameFN(filename string, outerOpts ...TransactionOption) ScriptOptsFunction {

	return func(opts ...TransactionOption) *OverflowScriptResult {

		for _, opt := range opts {
			outerOpts = append(outerOpts, opt)
		}
		return o.Script(filename, outerOpts...)
	}
}

func (o *OverflowState) TxFileNameFN(filename string, outerOpts ...TransactionOption) TransactionOptsFunction {

	return func(opts ...TransactionOption) *OverflowResult {

		for _, opt := range opts {
			outerOpts = append(outerOpts, opt)
		}
		return o.Tx(filename, outerOpts...)

	}
}

func (o *OverflowState) Tx(filename string, opts ...TransactionOption) *OverflowResult {
	return o.BuildInteraction(filename, "transaction", opts...).Send()
}

func (o *OverflowState) Script(filename string, opts ...TransactionOption) *OverflowScriptResult {
	interaction := o.BuildInteraction(filename, "script", opts...)

	osc := &OverflowScriptResult{Input: interaction}
	if interaction.Error != nil {
		osc.Err = interaction.Error
		return osc
	}

	filePath := fmt.Sprintf("%s/%s.cdc", interaction.BasePath, interaction.FileName)

	o.EmulatorLog.Reset()
	o.Log.Reset()

	result, err := o.Services.Scripts.Execute(
		interaction.TransactionCode,
		interaction.Arguments,
		filePath,
		o.Network)

	osc.Result = result
	osc.Err = err

	var logMessage []LogrusMessage
	dec := json.NewDecoder(o.Log)
	for {
		var doc LogrusMessage

		err := dec.Decode(&doc)
		if err == io.EOF {
			// all done
			break
		}
		if err != nil {
			panic(err)
		}

		logMessage = append(logMessage, doc)
	}

	o.EmulatorLog.Reset()
	o.Log.Reset()

	osc.Log = logMessage
	if osc.Err != nil {
		return osc
	}

	o.Logger.Info(fmt.Sprintf("%v Script run from path %s\n", emoji.Star, filePath))
	return osc
}

//shouls this be private?
func (o *OverflowState) BuildInteraction(filename string, interactionType string, opts ...TransactionOption) *FlowInteractionBuilder {

	path := o.TransactionBasePath
	if interactionType == "script" {
		path = o.ScriptBasePath
	}
	ftb := &FlowInteractionBuilder{
		Overflow:       o,
		MainSigner:     nil,
		Arguments:      []cadence.Value{},
		PayloadSigners: []*flowkit.Account{},
		GasLimit:       uint64(o.Gas),
		BasePath:       path,
		NamedArgs:      map[string]interface{}{},
	}

	if strings.Contains(filename, "transaction (") ||
		strings.Contains(filename, "transaction {") ||
		strings.Contains(filename, "transaction{") ||
		strings.Contains(filename, "transaction(") ||
		strings.Contains(filename, "transaction ") ||
		strings.Contains(filename, "pub fun main(") {
		ftb.TransactionCode = []byte(filename)
		ftb.FileName = "inline"
	} else {
		filePath := fmt.Sprintf("%s/%s.cdc", ftb.BasePath, filename)
		code, err := ftb.getContractCode(filePath)
		ftb.TransactionCode = code
		ftb.FileName = filename
		if err != nil {
			ftb.Error = err
			return ftb
		}
	}
	for _, opt := range opts {
		opt(ftb)
	}
	if ftb.Error != nil {
		return ftb
	}

	parseArgs, err := o.parseArguments(ftb.FileName, ftb.TransactionCode, ftb.NamedArgs)
	if err != nil {
		ftb.Error = err
		return ftb
	}
	ftb.Arguments = parseArgs
	return ftb
}

func Args(args ...interface{}) func(ftb *FlowInteractionBuilder) {

	return func(ftb *FlowInteractionBuilder) {
		if len(args)%2 != 0 {
			ftb.Error = fmt.Errorf("Please send in an even number of string : interface{} pairs")
			return
		}
		var i = 0
		for i < len(args) {
			key := args[0]
			value, labelOk := key.(string)
			if !labelOk {
				ftb.Error = fmt.Errorf("even parameters in Args needs to be strings")
			}
			ftb.NamedArgs[value] = args[1]
			i = i + 2
		}
	}
}

func ArgsM(args map[string]interface{}) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		for key, value := range args {
			ftb.NamedArgs[key] = value
		}
	}
}

/// Send an argument into a transaction
/// @param name: string the name of the parameter
/// @param value: the value of the argument, se below
///
/// The value is treated in the given way depending on type
///  - cadence.Value is sent as straight argument
///  - string argument are resolved into cadence.Value using flowkit
///  - ofther values are converted to string with %v and resolved into cadence.Value using flowkit
///  - if the type of the paramter is Address and the string you send in is a valid account in flow.json it will resolve
///
/// Examples:
///  If you want to send the UFix64 number "42.0" into a transaciton you have to use it as a string since %v of fmt.Sprintf will make it 42
func Arg(name string, value interface{}) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		ftb.NamedArgs[name] = value
	}
}

func DateTimeArg(name string, dateString string, timezone string) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		value, err := parseTime(dateString, timezone)
		if err != nil {
			ftb.Error = err
			return
		}

		//swallow the error since it will never happen here, we control the input
		amount, _ := cadence.NewUFix64(value)

		ftb.NamedArgs[name] = amount
	}
}

func Addresses(name string, value ...string) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		array := []cadence.Value{}

		for _, val := range value {
			account, err := ftb.Overflow.AccountE(val)
			if err != nil {
				address, err := HexToAddress(val)
				if err != nil {
					ftb.Error = errors.Wrap(err, fmt.Sprintf("%s is not an valid account name or an address", val))
					return
				}
				cadenceAddress := cadence.BytesToAddress(address.Bytes())
				array = append(array, cadenceAddress)
			} else {
				cadenceAddress := cadence.BytesToAddress(account.Address().Bytes())
				array = append(array, cadenceAddress)
			}
		}
		ftb.NamedArgs[name] = cadence.NewArray(array)
	}
}

func ProposeAs(proposer string) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		account, err := ftb.Overflow.AccountE(proposer)
		if err != nil {
			ftb.Error = err
			return
		}
		ftb.Proposer = account
	}
}

func ProposeAsServiceAccount() func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		key := ftb.Overflow.ServiceAccountName()
		account, _ := ftb.Overflow.State.Accounts().ByName(key)
		ftb.Proposer = account
	}
}

func SignProposeAndPayAs(signer string) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		account, err := ftb.Overflow.AccountE(signer)
		if err != nil {
			ftb.Error = err
			return
		}
		ftb.MainSigner = account
		ftb.Proposer = account
	}
}

func SignProposeAndPayAsServiceAccount() func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		key := ftb.Overflow.ServiceAccountName()
		account, _ := ftb.Overflow.State.Accounts().ByName(key)
		ftb.MainSigner = account
		ftb.Proposer = account
	}
}

func Gas(gas uint64) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		ftb.GasLimit = gas
	}
}

func EventFilter(filter OverFlowEventFilter) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		ftb.EventFilter = filter
	}
}

func IgnoreGlobalEventFilters() func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		ftb.IgnoreGlobalEventFilters = true
	}
}

func PayloadSigner(signer ...string) func(ftb *FlowInteractionBuilder) {
	return func(ftb *FlowInteractionBuilder) {
		for _, signer := range signer {
			account, err := ftb.Overflow.AccountE(signer)
			if err != nil {
				ftb.Error = err
				return
			}
			ftb.PayloadSigners = append(ftb.PayloadSigners, account)
		}
	}
}
