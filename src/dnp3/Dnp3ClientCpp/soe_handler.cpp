/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 *
 * soe_handler.cpp - ISOEHandler implementation: receives all data from the
 * DNP3 stack and enqueues Dnp3Value items for the MongoDB update thread.
 */

#include "soe_handler.h"

SOEHandler::SOEHandler(shared_ptr<DNP3Connection> conn) : conn(std::move(conn)) {}

void SOEHandler::BeginFragment(const ResponseInfo& info)
{
    Log.log(conn->name + " - Begin Fragment: " + (info.unsolicited ? "Unsolicited" : "solicited"),
        Logger::Level::Detailed);
}

void SOEHandler::EndFragment(const ResponseInfo& info)
{
    Log.log(conn->name + " - End Fragment", Logger::Level::Detailed);
}

// ---------------------------------------------------------------------------
// Generic value enqueue (template definition – all instantiations are below)
// ---------------------------------------------------------------------------

template<class T>
void SOEHandler::pushValue(int baseGroup, int group, int variation,
    const Indexed<T>& item, double value, const string& valueString,
    bool online, bool commLost, bool remoteForced, bool localForced,
    bool overrange, bool rollover, bool referenceError, bool transient)
{
    Dnp3Value out;
    out.address   = item.index;
    out.baseGroup = baseGroup;
    out.group     = group;
    out.variation = variation;
    out.value     = value;
    out.valueString = valueString;
    out.cot         = 20;
    out.serverTimestamp = nowMs();
    out.hasSourceTimestamp = item.value.time.value > 0;
    out.sourceTimestamp    = item.value.time.value;
    out.timeTagOk = item.value.time.quality == TimestampQuality::SYNCHRONIZED;

    Log.log(conn->name + " - Data Recv: addr=" + to_string(item.index)
        + " group=" + to_string(group) + " val=" + valueString
        + " time=" + to_string(item.value.time.value)
        + " qual=" + to_string(static_cast<int>(item.value.time.quality)),
        Logger::Level::Detailed);

    out.qOnline        = online;
    out.qCommLost      = commLost;
    out.qRemoteForced  = remoteForced;
    out.qLocalForced   = localForced;
    out.qOverrange     = overrange;
    out.qRollover      = rollover;
    out.qReferenceError = referenceError;
    out.qTransient     = transient;
    out.connNumber     = conn->protocolConnectionNumber;
    enqueueValue(out);
}

// ---------------------------------------------------------------------------
// ISOEHandler::Process overrides
// ---------------------------------------------------------------------------

void SOEHandler::Process(const HeaderInfo& info, const ICollection<Indexed<Binary>>& values)
    { processBinary(info, values); }
void SOEHandler::Process(const HeaderInfo& info, const ICollection<Indexed<DoubleBitBinary>>& values)
    { processDoubleBinary(info, values); }
void SOEHandler::Process(const HeaderInfo& info, const ICollection<Indexed<Analog>>& values)
    { processAnalog(info, values); }
void SOEHandler::Process(const HeaderInfo& info, const ICollection<Indexed<Counter>>& values)
    { processCounter(info, values, 20); }
void SOEHandler::Process(const HeaderInfo& info, const ICollection<Indexed<FrozenCounter>>& values)
    { processFrozenCounter(info, values); }
void SOEHandler::Process(const HeaderInfo& info, const ICollection<Indexed<BinaryOutputStatus>>& values)
    { processBinaryOutput(info, values); }
void SOEHandler::Process(const HeaderInfo& info, const ICollection<Indexed<AnalogOutputStatus>>& values)
    { processAnalogOutputStatus(info, values); }

void SOEHandler::Process(const HeaderInfo&, const ICollection<Indexed<OctetString>>&) {}
void SOEHandler::Process(const HeaderInfo&, const ICollection<Indexed<TimeAndInterval>>&) {}
void SOEHandler::Process(const HeaderInfo&, const ICollection<Indexed<BinaryCommandEvent>>&) {}
void SOEHandler::Process(const HeaderInfo&, const ICollection<Indexed<AnalogCommandEvent>>&) {}
void SOEHandler::Process(const HeaderInfo&, const ICollection<DNPTime>&) {}

// ---------------------------------------------------------------------------
// Private process methods
// ---------------------------------------------------------------------------

void SOEHandler::processBinary(const HeaderInfo& info, const ICollection<Indexed<Binary>>& values)
{
    Log.log(conn->name + " - Process Binary GV=" + to_string(static_cast<int>(info.gv))
        + " Count=" + to_string(values.Count()), Logger::Level::Detailed);
    try
    {
        values.ForeachItem([&](const Indexed<Binary>& item) {
            pushValue(1, 1, 0, item, item.value.value ? 1.0 : 0.0,
                item.value.value ? "true" : "false",
                (item.value.flags.value & static_cast<uint8_t>(BinaryQuality::ONLINE)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(BinaryQuality::COMM_LOST)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(BinaryQuality::REMOTE_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(BinaryQuality::LOCAL_FORCED)) != 0,
                false, false, false, false);
        });
    }
    catch (const exception& ex)
    {
        Log.log(conn->name + " - Exception in processBinary: " + string(ex.what()),
            Logger::Level::Detailed);
    }
}

void SOEHandler::processDoubleBinary(const HeaderInfo& info, const ICollection<Indexed<DoubleBitBinary>>& values)
{
    Log.log(conn->name + " - Process DoubleBinary GV=" + to_string(static_cast<int>(info.gv))
        + " Count=" + to_string(values.Count()), Logger::Level::Detailed);
    try
    {
        values.ForeachItem([&](const Indexed<DoubleBitBinary>& item) {
            const bool transient = item.value.value == DoubleBit::INTERMEDIATE
                                || item.value.value == DoubleBit::INDETERMINATE;
            const double val = (item.value.value == DoubleBit::DETERMINED_ON
                             || item.value.value == DoubleBit::INDETERMINATE) ? 1.0 : 0.0;
            pushValue(3, 3, 0, item, val, to_string(static_cast<int>(item.value.value)),
                (item.value.flags.value & static_cast<uint8_t>(DoubleBitBinaryQuality::ONLINE)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(DoubleBitBinaryQuality::COMM_LOST)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(DoubleBitBinaryQuality::REMOTE_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(DoubleBitBinaryQuality::LOCAL_FORCED)) != 0,
                false, false, false, transient);
        });
    }
    catch (const exception& ex)
    {
        Log.log(conn->name + " - Exception in processDoubleBinary: " + string(ex.what()),
            Logger::Level::Detailed);
    }
}

void SOEHandler::processAnalog(const HeaderInfo& info, const ICollection<Indexed<Analog>>& values)
{
    Log.log(conn->name + " - Process Analog GV=" + to_string(static_cast<int>(info.gv))
        + " Count=" + to_string(values.Count()), Logger::Level::Detailed);
    try
    {
        values.ForeachItem([&](const Indexed<Analog>& item) {
            pushValue(30, 30, 0, item, item.value.value, to_string(item.value.value),
                (item.value.flags.value & static_cast<uint8_t>(AnalogQuality::ONLINE)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogQuality::COMM_LOST)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogQuality::REMOTE_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogQuality::LOCAL_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogQuality::OVERRANGE)) != 0,
                false,
                (item.value.flags.value & static_cast<uint8_t>(AnalogQuality::REFERENCE_ERR)) != 0,
                false);
        });
    }
    catch (const exception& ex)
    {
        Log.log(conn->name + " - Exception in processAnalog: " + string(ex.what()),
            Logger::Level::Detailed);
    }
}

void SOEHandler::processCounter(const HeaderInfo& info, const ICollection<Indexed<Counter>>& values, int baseGroup)
{
    Log.log(conn->name + " - Process Counter GV=" + to_string(static_cast<int>(info.gv))
        + " Count=" + to_string(values.Count()), Logger::Level::Detailed);
    try
    {
        values.ForeachItem([&](const Indexed<Counter>& item) {
            pushValue(baseGroup, baseGroup, 0, item, item.value.value, to_string(item.value.value),
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::ONLINE)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::COMM_LOST)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::REMOTE_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::LOCAL_FORCED)) != 0,
                false,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::ROLLOVER)) != 0,
                false, false);
        });
    }
    catch (const exception& ex)
    {
        Log.log(conn->name + " - Exception in processCounter: " + string(ex.what()),
            Logger::Level::Detailed);
    }
}

void SOEHandler::processFrozenCounter(const HeaderInfo& info, const ICollection<Indexed<FrozenCounter>>& values)
{
    Log.log(conn->name + " - Process FrozenCounter GV=" + to_string(static_cast<int>(info.gv))
        + " Count=" + to_string(values.Count()), Logger::Level::Detailed);
    try
    {
        values.ForeachItem([&](const Indexed<FrozenCounter>& item) {
            pushValue(23, 23, 0, item, item.value.value, to_string(item.value.value),
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::ONLINE)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::COMM_LOST)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::REMOTE_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::LOCAL_FORCED)) != 0,
                false,
                (item.value.flags.value & static_cast<uint8_t>(CounterQuality::ROLLOVER)) != 0,
                false, false);
        });
    }
    catch (const exception& ex)
    {
        Log.log(conn->name + " - Exception in processFrozenCounter: " + string(ex.what()),
            Logger::Level::Detailed);
    }
}

void SOEHandler::processBinaryOutput(const HeaderInfo& info, const ICollection<Indexed<BinaryOutputStatus>>& values)
{
    Log.log(conn->name + " - Process BinaryOutputStatus GV=" + to_string(static_cast<int>(info.gv))
        + " Count=" + to_string(values.Count()), Logger::Level::Detailed);
    try
    {
        values.ForeachItem([&](const Indexed<BinaryOutputStatus>& item) {
            pushValue(10, 10, 0, item, item.value.value ? 1.0 : 0.0,
                item.value.value ? "true" : "false",
                (item.value.flags.value & static_cast<uint8_t>(BinaryOutputStatusQuality::ONLINE)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(BinaryOutputStatusQuality::COMM_LOST)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(BinaryOutputStatusQuality::REMOTE_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(BinaryOutputStatusQuality::LOCAL_FORCED)) != 0,
                false, false, false, false);
        });
    }
    catch (const exception& ex)
    {
        Log.log(conn->name + " - Exception in processBinaryOutput: " + string(ex.what()),
            Logger::Level::Detailed);
    }
}

void SOEHandler::processAnalogOutputStatus(const HeaderInfo& info, const ICollection<Indexed<AnalogOutputStatus>>& values)
{
    Log.log(conn->name + " - Process AnalogOutputStatus GV=" + to_string(static_cast<int>(info.gv))
        + " Count=" + to_string(values.Count()), Logger::Level::Detailed);
    try
    {
        values.ForeachItem([&](const Indexed<AnalogOutputStatus>& item) {
            pushValue(40, 40, 0, item, item.value.value, to_string(item.value.value),
                (item.value.flags.value & static_cast<uint8_t>(AnalogOutputStatusQuality::ONLINE)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogOutputStatusQuality::COMM_LOST)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogOutputStatusQuality::REMOTE_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogOutputStatusQuality::LOCAL_FORCED)) != 0,
                (item.value.flags.value & static_cast<uint8_t>(AnalogOutputStatusQuality::OVERRANGE)) != 0,
                false,
                (item.value.flags.value & static_cast<uint8_t>(AnalogOutputStatusQuality::REFERENCE_ERR)) != 0,
                false);
        });
    }
    catch (const exception& ex)
    {
        Log.log(conn->name + " - Exception in processAnalogOutputStatus: " + string(ex.what()),
            Logger::Level::Detailed);
    }
}

template<class T>
void SOEHandler::processAnalogCommand(const HeaderInfo&, const ICollection<Indexed<T>>& values, int variation)
{
    values.ForeachItem([&](const Indexed<T>& item) {
        Dnp3Value out;
        out.address     = item.index;
        out.baseGroup   = 41;
        out.group       = 41;
        out.variation   = variation;
        out.value       = static_cast<double>(item.value.value);
        out.valueString = to_string(out.value);
        out.serverTimestamp = nowMs();
        out.connNumber  = conn->protocolConnectionNumber;
        enqueueValue(out);
    });
}
