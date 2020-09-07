using libplctag.NativeImport;

namespace libplctag
{
    /// <summary>
    /// Events returned by the base libplctag library
    /// </summary>
    public enum Event
    {
        ReadStarted =       EVENT_CODES.PLCTAG_EVENT_READ_STARTED,
        ReadCompleted =     EVENT_CODES.PLCTAG_EVENT_READ_COMPLETED,
        WriteStarted =      EVENT_CODES.PLCTAG_EVENT_WRITE_STARTED,
        WriteCompleted =    EVENT_CODES.PLCTAG_EVENT_WRITE_COMPLETED,
        Aborted =           EVENT_CODES.PLCTAG_EVENT_ABORTED,
        Destroyed =         EVENT_CODES.PLCTAG_EVENT_DESTROYED
    }
}