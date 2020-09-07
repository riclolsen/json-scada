using libplctag.NativeImport;

namespace libplctag
{
    public enum Status
    {
        Pending =               STATUS_CODES.PLCTAG_STATUS_PENDING,
        Ok =                    STATUS_CODES.PLCTAG_STATUS_OK,
        ErrorAbort =            STATUS_CODES.PLCTAG_ERR_ABORT,
        ErrorBadConfig =        STATUS_CODES.PLCTAG_ERR_BAD_CONFIG,
        ErrorBadConnection =    STATUS_CODES.PLCTAG_ERR_BAD_CONNECTION,
        ErrorBadData =          STATUS_CODES.PLCTAG_ERR_BAD_DATA,
        ErrorBadDevice =        STATUS_CODES.PLCTAG_ERR_BAD_DEVICE,
        ErrorBadGateway =       STATUS_CODES.PLCTAG_ERR_BAD_GATEWAY,
        ErrorBadParam =         STATUS_CODES.PLCTAG_ERR_BAD_PARAM,
        ErrorBadReply =         STATUS_CODES.PLCTAG_ERR_BAD_REPLY,
        ErrorBadStatus =        STATUS_CODES.PLCTAG_ERR_BAD_STATUS,
        ErrorClose =            STATUS_CODES.PLCTAG_ERR_CLOSE,
        ErrorCreate =           STATUS_CODES.PLCTAG_ERR_CREATE,
        ErrorDuplicate =        STATUS_CODES.PLCTAG_ERR_DUPLICATE,
        ErrorEncode =           STATUS_CODES.PLCTAG_ERR_ENCODE,
        ErrorMutexDestroy =     STATUS_CODES.PLCTAG_ERR_MUTEX_DESTROY,
        ErrorMutexInit =        STATUS_CODES.PLCTAG_ERR_MUTEX_INIT,
        ErrorMutexLock =        STATUS_CODES.PLCTAG_ERR_MUTEX_LOCK,
        ErrorMutexUnlock =      STATUS_CODES.PLCTAG_ERR_MUTEX_UNLOCK,
        ErrorNotAllowed =       STATUS_CODES.PLCTAG_ERR_NOT_ALLOWED,
        ErrorNotFound =         STATUS_CODES.PLCTAG_ERR_NOT_FOUND,
        ErrorNotImplemented =   STATUS_CODES.PLCTAG_ERR_NOT_IMPLEMENTED,
        ErrorNoData =           STATUS_CODES.PLCTAG_ERR_NO_DATA,
        ErrorNoMatch =          STATUS_CODES.PLCTAG_ERR_NO_MATCH,
        ErrorNoMem =            STATUS_CODES.PLCTAG_ERR_NO_MEM,
        ErrorNoResources =      STATUS_CODES.PLCTAG_ERR_NO_RESOURCES,
        ErrorNullPtr =          STATUS_CODES.PLCTAG_ERR_NULL_PTR,
        ErrorOpen =             STATUS_CODES.PLCTAG_ERR_OPEN,
        ErrorOutOfBounds =      STATUS_CODES.PLCTAG_ERR_OUT_OF_BOUNDS,
        ErrorRead =             STATUS_CODES.PLCTAG_ERR_READ,
        ErrorRemoteErr =        STATUS_CODES.PLCTAG_ERR_REMOTE_ERR,
        ErrorThreadCreate =     STATUS_CODES.PLCTAG_ERR_THREAD_CREATE,
        ErrorThreadJoin =       STATUS_CODES.PLCTAG_ERR_THREAD_JOIN,
        ErrorTimeout =          STATUS_CODES.PLCTAG_ERR_TIMEOUT,
        ErrorTooLarge =         STATUS_CODES.PLCTAG_ERR_TOO_LARGE,
        ErrorTooSmall =         STATUS_CODES.PLCTAG_ERR_TOO_SMALL,
        ErrorUnsupported =      STATUS_CODES.PLCTAG_ERR_UNSUPPORTED,
        ErrorWinsock =          STATUS_CODES.PLCTAG_ERR_WINSOCK,
        ErrorWrite =            STATUS_CODES.PLCTAG_ERR_WRITE,
        ErrorPartial =          STATUS_CODES.PLCTAG_ERR_PARTIAL,
        ErrorBusy =             STATUS_CODES.PLCTAG_ERR_BUSY
    }
}