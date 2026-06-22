/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 */

#pragma once

#include "dnp3client.h"

// SOEHandler receives all data indications from the DNP3 master stack and
// enqueues them for the MongoDB update thread.
class SOEHandler final : public ISOEHandler
{
public:
    explicit SOEHandler(shared_ptr<DNP3Connection> conn);

    void BeginFragment(const ResponseInfo& info) override;
    void EndFragment(const ResponseInfo& info) override;

    void Process(const HeaderInfo& info, const ICollection<Indexed<Binary>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<DoubleBitBinary>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<Analog>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<Counter>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<FrozenCounter>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<BinaryOutputStatus>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<AnalogOutputStatus>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<OctetString>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<TimeAndInterval>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<BinaryCommandEvent>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<Indexed<AnalogCommandEvent>>& values) override;
    void Process(const HeaderInfo& info, const ICollection<DNPTime>& values) override;

private:
    template<class T>
    void pushValue(int baseGroup, int group, int variation, const Indexed<T>& item,
        double value, const string& valueString,
        bool online, bool commLost, bool remoteForced, bool localForced,
        bool overrange, bool rollover, bool referenceError, bool transient);

    template<class T>
    void processAnalogCommand(const HeaderInfo&, const ICollection<Indexed<T>>& values, int variation);

    void processBinary(const HeaderInfo& info, const ICollection<Indexed<Binary>>& values);
    void processDoubleBinary(const HeaderInfo& info, const ICollection<Indexed<DoubleBitBinary>>& values);
    void processAnalog(const HeaderInfo& info, const ICollection<Indexed<Analog>>& values);
    void processCounter(const HeaderInfo& info, const ICollection<Indexed<Counter>>& values, int baseGroup);
    void processFrozenCounter(const HeaderInfo& info, const ICollection<Indexed<FrozenCounter>>& values);
    void processBinaryOutput(const HeaderInfo& info, const ICollection<Indexed<BinaryOutputStatus>>& values);
    void processAnalogOutputStatus(const HeaderInfo& info, const ICollection<Indexed<AnalogOutputStatus>>& values);

    shared_ptr<DNP3Connection> conn;
};
