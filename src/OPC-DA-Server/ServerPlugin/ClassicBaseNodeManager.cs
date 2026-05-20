#region Copyright (c) 2011-2023 Technosoftware GmbH. All rights reserved
//-----------------------------------------------------------------------------
// Copyright (c) 2011-2023 Technosoftware GmbH. All rights reserved
// Web: https://technosoftware.com 
// 
// License: 
// 
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT
//-----------------------------------------------------------------------------
#endregion Copyright (c) 2011-2023 Technosoftware GmbH. All rights reserved

#region Using Directives

using System;
using System.Xml;
using System.Threading;

#endregion

namespace ServerPlugin
{

    #region Structures, Enumerations and Classes

    //-------------------------------------------------------------------------
    // These structures, enumerations and classes match the OPC specifications 
    // or the server EXE file.
    //-------------------------------------------------------------------------

    /// <summary>
    /// Declares constants for common XML Schema and OPC namespaces.
    /// </summary>
    public class Namespaces
    {
        /// <summary>XML Schema</summary>
        public const string XmlSchema = "http://www.w3.org/2001/XMLSchema";
        /// <summary>XML Schema Instance</summary>
        public const string XmlSchemaInstance = "http://www.w3.org/2001/XMLSchema-instance";
        /// <summary>OPC Alarms &amp; Events</summary>
        public const string OpcAlarmAndEvents = "http://opcfoundation.org/AlarmAndEvents/";
        /// <summary>OPC Complex Data</summary>
        public const string OpcComplexData = "http://opcfoundation.org/ComplexData/";
        /// <summary>OPC Data Exchange</summary>
        public const string OpcDataExchange = "http://opcfoundation.org/DataExchange/";
        /// <summary>OPC Data Access</summary>
        public const string OpcDataAccess = "http://opcfoundation.org/DataAccess/";
        /// <summary>OPC Historical Data Access</summary>
        public const string OpcHistoricalDataAccess = "http://opcfoundation.org/HistoricalDataAccess/";
        /// <summary>OPC Binary 1.0</summary>
        public const string OpcBinary = "http://opcfoundation.org/OPCBinary/1.0/";
        /// <summary>OPC XML-DA 1.0</summary>
        public const string OpcDataAccessXml10 = "http://opcfoundation.org/webservices/XMLDA/1.0/";
        /// <summary>OPC UA 1.0</summary>
        public const string OpcUa10 =   "http://opcfoundation.org/webservices/UA/1.0/";
    }


    /// <summary>
    /// Represents the log level.
    /// </summary>
    public enum LogLevel
    {
        /// <summary>
        /// Represents the Trace log level. This log level is mostly
        /// ntended to be used in the debug and development process.
        /// </summary>
        Trace = 0,

        /// <summary>
        /// Represents the Debug log level. This log level is mostly
        /// intended to be used in the debug and development process.
        /// </summary>
        Debug = 1,

        /// <summary>
        /// Represents the Info log level. This log level is intended
        /// to track the general progress of applications at a
        /// coarse-grained level.
        /// </summary>
        Info = 2,

        /// <summary>
        /// Represents the Warning log level. This log level designates
        /// potentially harmful events or situations.
        /// </summary>
        Warning = 3,

        /// <summary>
        /// Represents the Error log level. This log level designates
        /// error events or situations which are not critical to the
        /// entire system. This log level thus describes recoverable
        /// or less important errors.
        /// </summary>
        Error = 4,

        /// <summary>
        /// Represents the Alarm log level. This log level designates
        /// error events or situations which are critical to the
        /// entire system. This log level thus describes recoverable
        /// or important errors.
        /// </summary>
        Alarm = 5,

        /// <summary>
        /// Represents the Fatal log level. This log level designates
        /// errors which are not recoverable and eventually stop the
        /// system or application from working.
        /// </summary>
        Fatal = 6,

        /// <summary>
        /// Represents the Disabled log level. This log level disable logging.
        /// </summary>
        Disabled = 7,
    };

    /// <summary>
    /// Server information structure returned by OnGetDaServerDefinition
    /// and OnGetAeServerRegistryDefinition.
    /// </summary>
    public struct ClassicServerDefinition
    {
        /// <summary>
        /// The DCOM server is registered with this CLSID.
        /// </summary>
        public string ClsIdServer;

        /// <summary>
        ///  The DCOM server application is registered with this CLSID in the AppID section.
        /// </summary>
        public string ClsIdApp;
        
        /// <summary>
        /// Version independent ProgID of the DCOM server.
        /// </summary>
        public string PrgIdServer;
        
        /// <summary>
        /// ProgID of the current DCOM server version.
        /// </summary>
        public string PrgIdCurrServer;
        
        /// <summary>
        /// Version independent friendly name of the DCOM server.
        /// </summary>
        public string ServerName;
        
        /// <summary>
        /// Version independent friendly name of the current DCOM server version. 
        /// </summary>
        public string CurrServerName;
        
        /// <summary>
        /// Server vendor name.
        /// </summary>
        public string CompanyName;
    };

    /// <summary>
    /// The set of possible server states. Corresponds to the OPC UA ServerState
    /// </summary>
    public enum ServerState
    {
        /// <summary>
        /// The server is shutting down.
        /// </summary>
        Shutdown = 0,

        /// <summary>
        /// The server is running normally.
        /// </summary>
        Running = 1,

        /// <summary>
        /// The server is not functioning due to a fatal error.
        /// </summary>
        Failed = 2,

        /// <summary>
        /// The server cannot load its configuration information.
        /// </summary>
        NoConfiguration = 3,

        /// <summary>
        /// The server has halted all communication with the underlying hardware.
        /// </summary>
        Suspended = 4,

        /// <summary>
        /// The server is disconnected from the underlying hardware.
        /// </summary>
        Test = 5,

        /// <summary>
        /// The server cannot communicate with the underlying hardware.
        /// </summary>
        CommunicationFault = 6,

        /// <summary>
        /// The server state is not known.
        /// </summary>
        Unknown = 7,
    }


    /// <summary>Contains information about an OPC DA Group.</summary>
    public struct DaGroupState
    {
        /// <summary>
        /// Name of the group.
        /// </summary>
        public string GroupName;

        /// <summary>
        /// The update rate at which the server can refresh the group data in msec.
        /// </summary>
        public long UpdateRate;

        /// <summary>
        /// Handle for this group provided by the client and used during refresh or read of group
        /// within callback functions to identify the group.
        /// </summary>
        public long ClientGroupHandle;

        /// <summary>
        ///The deadband in percent.
        /// </summary>
        public float PercentDeadband;

        /// <summary>
        /// The Locale ID.
        /// </summary>
        public long LocaleId;

        /// <summary>
        /// Tells whether IOPCDataCallback::OnDataChange callbacks are disabled or enabled.
        /// </summary>
        public bool DataChangeEnabled;

    };

    /// <summary>Contains information about an OPC DA Item.</summary>
    public struct DaItemState
    {
        /// <summary>
        /// Name of the item.
        /// </summary>
        public string ItemName;

        /// <summary>
        /// Recommendation to the server on 'how to get the data'
        /// </summary>
        public String AccessPath;

        /// <summary>
        /// Item access rights.
        /// </summary>
        public DaAccessRights AccessRights;

        /// <summary>
        /// Handle of the device item.
        /// </summary>
        public IntPtr DeviceItemHandle;

    };

    /// <summary>
    /// Contains the value for a single item. passed in WriteItems()
    /// </summary>
    public class DaDeviceItemValue
    {
        /// <summary>
        /// 	<para>Application handle of the item.</para>
        /// 	<para>This handle is used between the generic server and the customization module
        ///     to identify the items. The customization module should define the handle so that it
        ///     allows quick access to the data structures needed in handling the requests.</para>
        /// </summary>
        public IntPtr DeviceItemHandle { get; set; }

        /// <summary>
        /// 	<para>Value to be written to the device.</para>
        /// 	<para>The item value must alway be written to the generic server cache in the
        ///     canonical data type defined when the item was added.</para>
        /// </summary>
        public object Value { get; set; }
        
        /// <summary>
        /// 	<para>Item quality.</para>
        /// 	<para>The quality value is according the OPC DA specification.</para>
        /// </summary>
        public short Quality { get; set; }
        
        /// <summary>
        /// Indicates that the item quality is specified.
        /// </summary>
        public bool QualitySpecified { get; set; }
        
        /// <summary>
        /// Timestamp for this item value.
        /// </summary>
        public DateTime Timestamp { get; set; }
        
        /// <summary>
        /// Indicates that the timestamp is specified.
        /// </summary>
        public bool TimestampSpecified { get; set; }
    }

    /// <summary>
    /// Contains a unique identifier for a property.
    /// </summary>
    public struct DaPropertyId
    { 
        #region Constructors, Destructor, Initialization

        /// <summary>
        /// Initializes a property identified by a qualified name.
        /// </summary>
        public DaPropertyId(XmlQualifiedName name) : this()
        { QualifiedName = name; Code = 0; }

        /// <summary>
        /// Initializes a property identified by an integer.
        /// </summary>
        public DaPropertyId(int code) : this()
        { QualifiedName = null; Code = code; }

        /// <summary>
        /// Initializes a property identified by a property description.
        /// </summary>
        public DaPropertyId(string name, int code, string ns) : this()
        { QualifiedName = new XmlQualifiedName(name, ns); Code = code; }

        #endregion

        #region Properties

        /// <summary>
        /// Used for properties identified by a qualified name.
        /// </summary>
        public XmlQualifiedName QualifiedName { get; private set; }

        /// <summary>
        /// Used for properties identified by a integer.
        /// </summary>
        public int Code { get; private set; }

        #endregion

        #region Operators

        /// <summary>
        /// Returns true if the objects are equal.
        /// </summary>
        public static bool operator ==(DaPropertyId a, DaPropertyId b)
        {
            return a.Equals(b);
        }

        /// <summary>
        /// Returns true if the objects are not equal.
        /// </summary>
        public static bool operator !=(DaPropertyId a, DaPropertyId b)
        {
            return !a.Equals(b);
        }

        #endregion

        #region Object Member Overrides
        
        /// <summary>
        /// Returns true if the target object is equal to the object.
        /// </summary>
        public override bool Equals(object target)
        {
            if (target is DaPropertyId)
            {
                var propertyId = (DaPropertyId)target;

                // compare by integer if both specify valid integers.
                if (propertyId.Code != 0 && Code != 0)
                {
                    return (propertyId.Code == Code);
                }

                // compare by name if both specify valid names.
                if (propertyId.QualifiedName != null && QualifiedName != null)
                {
                    return (propertyId.QualifiedName == QualifiedName);
                }
            }

            return false;
        }

        /// <summary>
        /// Returns a useful hash code for the object.
        /// </summary>
        public override int GetHashCode()
        {
            if (Code != 0) return Code.GetHashCode();
            if (QualifiedName != null) return QualifiedName.GetHashCode();
            return base.GetHashCode();
        }

        /// <summary>
        /// Converts the property id to a string.
        /// </summary>
        public override string ToString()
        {
            if (QualifiedName != null && Code != 0) return String.Format("{0} ({1})", QualifiedName.Name, Code);
            if (QualifiedName != null) return QualifiedName.Name;
            if (Code != 0) return String.Format("{0}", Code);
            return "";
        }
        
        #endregion
    }

    /// <summary>
    /// Defines identifiers for well-known properties.
    /// </summary>
    public class DaProperty
    {
        /// <summary>
        /// 	<para>Item Canonical DataType</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        public static readonly DaPropertyId DataType = new DaPropertyId("dataType", 1, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Item Value</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        /// <remarks>
        /// Note the type of value returned is as indicated by the "Item Canonical DataType"
        /// and depends on the item. This will behave like a read from DEVICE.
        /// </remarks>
        public static readonly DaPropertyId Value = new DaPropertyId("value", 2, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Item Quality</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        /// <remarks>(OPCQUALITY stored in an I2). This will behave like a read from DEVICE.</remarks>
        public static readonly DaPropertyId Quality = new DaPropertyId("quality", 3, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Item Timestamp</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        /// <remarks>
        /// (will be converted from FILETIME). This will behave like a read from
        /// DEVICE.
        /// </remarks>
        public static readonly DaPropertyId Timestamp = new DaPropertyId("timestamp", 4, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Item Access Rights</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        /// <remarks>(OPCACCESSRIGHTS stored in an I4)</remarks>
        public static readonly DaPropertyId AccessRights = new DaPropertyId("accessRights", 5, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Server Scan Rate</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        /// <remarks>
        /// In Milliseconds. This represents the fastest rate at which the server could
        /// obtain data from the underlying data source. The nature of this source is not defined
        /// but is typically a DCS system, a SCADA system, a PLC via a COMM port or network, a
        /// Device Network, etc. This value generally represents the �best case� fastest
        /// RequestedUpdateRate which could be used if this item were added to an OPCGroup.<br/>
        /// The accuracy of this value (the ability of the server to attain �best case�
        /// performance) can be greatly affected by system load and other factors.
        /// </remarks>
        public static readonly DaPropertyId ScanRate = new DaPropertyId("scanRate", 6, Namespaces.OpcDataAccess);
        /// <remarks>
        /// 	<para>Indicate the type of Engineering Units (EU) information (if any) contained in
        ///     EUINFO.</para>
        /// 	<list type="bullet">
        /// 		<item>
        ///             0 - No EU information available (EUINFO will be VT_EMPTY).<br/>
        ///             Items added to the generic server cache with the function
        ///             <see cref="ClassicBaseNodeManager.AddItem">AddItem</see> are items with no EU
        ///             information.
        ///         </item>
        /// 		<item>
        ///             1 - Analog - EUINFO will contain a SAFEARRAY of exactly two doubles
        ///             (VT_ARRAY | VT_R8) corresponding to the LOW and HI EU range.<br/>
        ///             Items added to the generic with the function
        ///             <see cref="ClassicBaseNodeManager.AddAnalogItem">AddAnalogItem</see> use this
        ///             enginieering unit.
        ///         </item>
        /// 		<item>2 - Enumerated - EUINFO will contain a SAFEARRAY of strings (VT_ARRAY |
        ///         VT_BSTR) which contains a list of strings (Example: �OPEN�, �CLOSE�, �IN
        ///         TRANSIT�, etc.) corresponding to sequential numeric values (0, 1, 2,
        ///         etc.)</item>
        /// 	</list>
        /// </remarks>
        /// <summary>
        /// 	<para>Item EU Type</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        public static readonly DaPropertyId EuType = new DaPropertyId("euType", 7, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Item EUInfo</para>
        /// 	<para>Automatically handled by the generic server.</para>
        /// </summary>
        /// <value>
        /// 	<para>
        ///         If EUTYPE is �Analog� EUINFO will contain a SAFEARRAY of exactly two doubles
        ///         (VT_ARRAY | VT_R8) corresponding to the LOW and HI EU range. Items added to the
        ///         generic with the function
        ///         <see cref="ClassicBaseNodeManager.AddAnalogItem">AddAnalogItem</see> automatically have
        ///         this EUINFO.
        ///     </para>
        /// 	<para>If EUTYPE is �Enumerated� - EUINFO will contain a SAFEARRAY of strings
        ///     (VT_ARRAY | VT_BSTR) which contains a list of strings (Example: �OPEN�, �CLOSE�,
        ///     �IN TRANSIT�, etc.) corresponding to sequential numeric values (0, 1, 2,
        ///     etc.)</para>
        /// </value>
        public static readonly DaPropertyId EuInfo = new DaPropertyId("euInfo", 8, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>EU Units</para>
        /// 	<para>e.g. "DEGC" or "GALLONS"</para>
        /// </summary>
        public static readonly DaPropertyId EngineeringUnits = new DaPropertyId("engineeringUnits", 100, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Item Description</para>
        /// 	<para>e.g. "Evaporator 6 Coolant Temp"</para>
        /// </summary>
        public static readonly DaPropertyId Description = new DaPropertyId("description", 101, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>High EU</para>
        /// 	<para>Present only for 'analog' data. This represents the highest value likely to
        ///     be obtained in normal operation and is intended for such use as automatically
        ///     scaling a bargraph display.</para>
        /// 	<para>e.g. 1400.0</para>
        /// </summary>
        public static readonly DaPropertyId HighEu = new DaPropertyId("highEU", 102, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Low EU</para>
        /// 	<para>Present only for 'analog' data. This represents the lowest value likely to be
        ///     obtained in normal operation and is intended for such use as automatically scaling
        ///     a bargraph display.</para>
        /// 	<para>e.g. -200.0</para>
        /// </summary>
        public static readonly DaPropertyId LowEu = new DaPropertyId("lowEU", 103, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>High Instrument Range</para>
        /// 	<para>Present only for �analog� data. This represents the highest value that can be
        ///     returned by the instrument.</para>
        /// 	<para>e.g. 9999.9</para>
        /// </summary>
        public static readonly DaPropertyId HighIr = new DaPropertyId("highIR", 104, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Low Instrument Range</para>
        /// 	<para>Present only for �analog� data. This represents the lowest value that can be
        ///     returned by the instrument.</para>
        /// 	<para>e.g. -9999.9</para>
        /// </summary>
        public static readonly DaPropertyId LowIr = new DaPropertyId("lowIR", 105, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Contact Close Label</para>
        /// 	<para>Present only for �discrete' data. This represents a string to be associated
        ///     with this contact when it is in the closed (non-zero) state</para>
        /// 	<para>e.g. "RUN", "CLOSE", "ENABLE", "SAFE" ,etc.</para>
        /// </summary>
        public static readonly DaPropertyId CloseLabel = new DaPropertyId("closeLabel", 106, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Contact Open Label</para>
        /// 	<para>Present only for �discrete' data. This represents a string to be associated
        ///     with this contact when it is in the open (zero) state</para>
        /// 	<para>e.g. "STOP", "OPEN", "DISABLE", "UNSAFE" ,etc.</para>
        /// </summary>
        public static readonly DaPropertyId OpenLabel = new DaPropertyId("openLabel", 107, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Item Timezone</para>
        /// 	<para>The difference in minutes between the items UTC Timestamp and the local time
        ///     in which the item value was obtained.</para>
        /// </summary>
        /// <remarks>
        /// See the OPCGroup TimeBias property. Also see the WIN32 TIME_ZONE_INFORMATION
        /// structure.
        /// </remarks>
        public static readonly DaPropertyId TimeZone = new DaPropertyId("timeZone", 108, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Condition Status</para>
        /// 	<para>The current alarm or condition status associated with the Item<br/>
        ///     e.g. "NORMAL", "ACTIVE", "HI ALARM", etc</para>
        /// </summary>
        public static readonly DaPropertyId ConditionStatus = new DaPropertyId("conditionStatus", 300, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Alarm Quick Help</para>
        /// 	<para>A short text string providing a brief set of instructions for the operator to
        ///     follow when this alarm occurs.</para>
        /// </summary>
        public static readonly DaPropertyId AlarmQuickHelp = new DaPropertyId("alarmQuickHelp", 301, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Alarm Area List</para>
        /// 	<para>An array of stings indicating the plant or alarm areas which include this
        ///     ItemID.</para>
        /// </summary>
        public static readonly DaPropertyId AlarmAreaList = new DaPropertyId("alarmAreaList", 302, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Primary Alarm Area</para>
        /// 	<para>A string indicating the primary plant or alarm area including this
        ///     ItemID</para>
        /// </summary>
        public static readonly DaPropertyId PrimaryAlarmArea = new DaPropertyId("primaryAlarmArea", 303, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Condition Logic</para>
        /// 	<para>An arbitrary string describing the test being performed.</para>
        /// 	<para>e.g. "High Limit Exceeded" or "TAG.PV &gt;= TAG.HILIM"</para>
        /// </summary>
        public static readonly DaPropertyId ConditionLogic = new DaPropertyId("conditioLogic", 304, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Limit Exceeded</para>
        /// 	<para>For multistate alarms, the condition exceeded</para>
        /// 	<para>e.g. HIHI, HI, LO, LOLO</para>
        /// </summary>
        public static readonly DaPropertyId LimitExceeded = new DaPropertyId("limitExceeded", 305, Namespaces.OpcDataAccess);
        /// <summary>Deadband</summary>
        public static readonly DaPropertyId Deadband = new DaPropertyId("deadband", 306, Namespaces.OpcDataAccess);
        /// <summary>HiHi limit</summary>
        public static readonly DaPropertyId HiHiLimit = new DaPropertyId("hihiLimit", 307, Namespaces.OpcDataAccess);
        /// <summary>Hi Limit</summary>
        public static readonly DaPropertyId HiLimit = new DaPropertyId("hiLimit", 308, Namespaces.OpcDataAccess);
        /// <summary>Lo Limit</summary>
        public static readonly DaPropertyId LoLimit = new DaPropertyId("loLimit", 309, Namespaces.OpcDataAccess);
        /// <summary>LoLo Limit</summary>
        public static readonly DaPropertyId LoLoLimit = new DaPropertyId("loloLimit", 310, Namespaces.OpcDataAccess);
        /// <summary>Rate of Change Limit</summary>
        public static readonly DaPropertyId RateChangeLimit = new DaPropertyId("rangeOfChangeLimit", 311, Namespaces.OpcDataAccess);
        /// <summary>Deviation Limit</summary>
        public static readonly DaPropertyId DeviationLimit = new DaPropertyId("deviationLimit", 312, Namespaces.OpcDataAccess);
        /// <summary>
        /// 	<para>Sound File</para>
        /// 	<para>e.g. C:\MEDIA\FIC101.WAV, or .MID</para>
        /// </summary>
        public static readonly DaPropertyId SoundFile = new DaPropertyId("soundFile", 313, Namespaces.OpcDataAccess);
    }
    
    /// <summary>
    ///     <para>Defines possible item access rights.</para>
    ///     <para align="left">Indicates if this item is read only, write only or read/write.
    ///     This is NOT related to security but rather to the nature of the underlying
    ///     device.</para>
    /// </summary>
    public enum DaAccessRights
    {
        /// <summary>The access rights for this item are unknown.</summary>
        Unknown = 0x00,
        /// <summary>The client can read the data item's value.</summary>
        Readable = 0x01,
        /// <summary>The client can change the data item's value.</summary>
        Writable = 0x02,
        /// <summary>The client can read and change the data item's value.</summary>
        ReadWritable = 0x03
    }

    /// <summary><para>Defines possible item engineering unit types</para></summary>
    public enum DaEuType
    {
        /// <summary>No engineering unit information available</summary>
        NoEnum = 0x00,
        /// <summary>
        /// Analog engineering unit - will contain a SAFEARRAY of exactly two doubles
        /// (VT_ARRAY | VT_R8) corresponding to the LOW and HI EU range.
        /// </summary>
        Analog = 0x01,
        /// <summary>
        /// Enumerated enginnering unit - will contain a SAFEARRAY of strings (VT_ARRAY |
        /// VT_BSTR) which contains a list of strings (Example: �OPEN�, �CLOSE�, �IN TRANSIT�,
        /// etc.) corresponding to sequential numeric values (0, 1, 2, etc.)
        /// </summary>
        Enumerated = 0x02
    }

    /// <summary>
    ///     <para>Defines the possible quality status bits.</para>
    ///     <para>These flags represent the quality state for an item's data value. This is
    ///     intended to be similar to but slightly simpler than the Fieldbus Data Quality
    ///     Specification (section 4.4.1 in the H1 Final Specifications). This design makes it
    ///     fairly easy for both servers and client applications to determine how much
    ///     functionality they want to implement.</para>
    /// </summary>
    public enum DaQualityBits
    {
        /// <summary>The Quality of the value is Good.</summary>
        Good = 0x000000C0,
        /// <summary>The value has been Overridden. Typically this means the input has been disconnected and a manually entered value has been 'forced'.</summary>
        GoodLocalOverride = 0x000000D8,
        /// <summary>The value is bad but no specific reason is known.</summary>
        Bad = 0x00000000,
        /// <summary>
        /// There is some server specific problem with the configuration. For example the
        /// item in question has been deleted from the configuration.
        /// </summary>
        BadConfigurationError = 0x00000004,
        /// <summary>
        /// The input is required to be logically connected to something but is not. This
        /// quality may reflect that no value is available at this time, for reasons like the value
        /// may have not been provided by the data source.
        /// </summary>
        BadNotConnected = 0x00000008,
        /// <summary>A device failure has been detected.</summary>
        BadDeviceFailure = 0x0000000c,
        /// <summary>
        /// A sensor failure had been detected (the �Limits� field can provide additional
        /// diagnostic information in some situations).
        /// </summary>
        BadSensorFailure = 0x00000010,
        /// <summary>
        /// Communications have failed. However, the last known value is available. Note that
        /// the �age� of the value may be determined from the time stamp in the item state.
        /// </summary>
        BadLastKnownValue = 0x00000014,
        /// <summary>Communications have failed. There is no last known value is available.</summary>
        BadCommFailure = 0x00000018,
        /// <summary>
        /// The block is off scan or otherwise locked. This quality is also used when the
        /// active state of the item or the group containing the item is InActive.
        /// </summary>
        BadOutOfService = 0x0000001C,
        /// <summary>
        /// After Items are added to a group, it may take some time for the server to
        /// actually obtain values for these items. In such cases the client might perform a read
        /// (from cache), or establish a ConnectionPoint based subscription and/or execute a
        /// Refresh on such a subscription before the values are available. This substatus is only
        /// available from OPC DA 3.0 or newer servers.
        /// </summary>
        BadWaitingForInitialData = 0x00000020,
        /// <summary>There is no specific reason why the value is uncertain.</summary>
        Uncertain = 0x00000040,
        /// <summary>
        /// Whatever was writing this value has stopped doing so. The returned value should
        /// be regarded as �stale�. Note that this differs from a BAD value with Substatus
        /// badLastKnownValue (Last Known Value). That status is associated specifically with a
        /// detectable communications error on a �fetched� value. This error is associated with the
        /// failure of some external source to �put� something into the value within an acceptable
        /// period of time. Note that the �age� of the value can be determined from the time stamp
        /// in the item state.
        /// </summary>
        UncertainLastUsableValue = 0x00000044,
        /// <summary>
        /// Either the value has �pegged� at one of the sensor limits (in which case the
        /// limit field should be set to low or high) or the sensor is otherwise known to be out of
        /// calibration via some form of internal diagnostics (in which case the limit field should
        /// be none).
        /// </summary>
        UncertainSensorNotAccurate = 0x00000050,
        /// <summary>
        /// The returned value is outside the limits defined for this parameter. Note that in
        /// this case (per the Fieldbus Specification) the �Limits� field indicates which limit has
        /// been exceeded but does NOT necessarily imply that the value cannot move farther out of
        /// range.
        /// </summary>
        UncertainEuExceeded = 0x00000054,
        /// <summary>
        /// The value is derived from multiple sources and has less than the required number
        /// of Good sources.
        /// </summary>
        UncertainSubNormal = 0x00000058
    }

    /// <summary>
    ///     <para>Defines the possible limit status bits.</para>
    ///     <para>The Limit Field is valid regardless of the Quality and Substatus. In some
    ///     cases such as Sensor Failure it can provide useful diagnostic information.</para>
    /// </summary>
    public enum DaLimitBits
    {
        /// <summary>The value is free to move up or down</summary>
        None = 0x0,
        /// <summary>The value has �pegged� at some lower limit</summary>
        Low = 0x1,
        /// <summary>The value has �pegged� at some high limit</summary>
        High = 0x2,
        /// <summary>The value is a constant and cannot move</summary>
        Constant = 0x3
    }

    /// <summary>
    /// Defines bit masks for the quality.
    /// </summary>
    public enum DaQualityMasks
    {
        /// <summary>Quality related bits</summary>
        QualityMask = +0x00FC,
        /// <summary>Limit related bits</summary>
        LimitMask = +0x0003,
        /// <summary>Vendor specific bits</summary>
        VendorMask = -0x00FD
    }

    /// <summary>
    /// Contains the quality field for an item value.
    /// </summary>
    [Serializable]
    public struct DaQuality
    {
        #region Constructors, Destructor, Initialization

        /// <summary>
        /// Initializes the object with the specified quality.
        /// </summary>
        public DaQuality(DaQualityBits quality)
        {
            qualityBits_ = quality;
            limitBits_ = DaLimitBits.None;
            vendorBits_ = 0;
        }

        /// <summary>
        /// Initializes the object from the contents of a 16 bit integer.
        /// </summary>
        public DaQuality(short code)
        {
            qualityBits_ = (DaQualityBits)(code & (short)DaQualityMasks.QualityMask);
            limitBits_ = (DaLimitBits)(code & (short)DaQualityMasks.LimitMask);
            vendorBits_ = (byte)((code & (short)DaQualityMasks.VendorMask) >> 8);
        }

        #endregion

        /// <summary>
        /// The value in the quality bits field.
        /// </summary>
        public DaQualityBits QualityBits
        {
            get { return qualityBits_; }
            set { qualityBits_ = value; }
        }

        /// <summary>
        /// The value in the limit bits field.
        /// </summary>
        public DaLimitBits LimitBits
        {
            get { return limitBits_; }
            set { limitBits_ = value; }
        }

        /// <summary>
        /// The value in the quality bits field.
        /// </summary>
        public byte VendorBits
        {
            get { return vendorBits_; }
            set { vendorBits_ = value; }
        }

        /// <summary>
        /// Returns the quality as a 16 bit integer.
        /// </summary>
        public short Code
        {
            get
            {
                ushort code = 0;

                code |= (ushort)QualityBits;
                code |= (ushort)LimitBits;
                code |= (ushort)(VendorBits << 8);

                return (code <= Int16.MaxValue) ? (short)code : (short)-((UInt16.MaxValue + 1 - code));
            }
        }

        /// <summary>
        /// Initializes the quality from a 16 bit integer.
        /// </summary>
        public void SetCode(short code)
        {
            qualityBits_ = (DaQualityBits)(code & (short)DaQualityMasks.QualityMask);
            limitBits_ = (DaLimitBits)(code & (short)DaQualityMasks.LimitMask);
            vendorBits_ = (byte)((code & (short)DaQualityMasks.VendorMask) >> 8);
        }

        /// <summary>
        /// Returns true if the objects are equal.
        /// </summary>
        public static bool operator ==(DaQuality a, DaQuality b)
        {
            return a.Equals(b);
        }

        /// <summary>
        /// Returns true if the objects are not equal.
        /// </summary>
        public static bool operator !=(DaQuality a, DaQuality b)
        {
            return !a.Equals(b);
        }

        #region Object Member Overrides
        /// <summary>
        /// Converts a quality to a string with the format: 'quality[limit]:vendor'.
        /// </summary>
        public override string ToString()
        {
            string text = null;

            switch (QualityBits)
            {
                case DaQualityBits.Good:
                    text += "(Good";
                    break;
                case DaQualityBits.GoodLocalOverride:
                    text += "(Good:Local Override";
                    break;
                case DaQualityBits.Bad:
                    text += "(Bad";
                    break;
                case DaQualityBits.BadConfigurationError:
                    text += "(Bad:Configuration Error";
                    break;
                case DaQualityBits.BadNotConnected:
                    text += "(Bad:Not Connected";
                    break;
                case DaQualityBits.BadDeviceFailure:
                    text += "(Bad:Device Failure";
                    break;
                case DaQualityBits.BadSensorFailure:
                    text += "(Bad:Sensor Failure";
                    break;
                case DaQualityBits.BadLastKnownValue:
                    text += "(Bad:Last Known Value";
                    break;
                case DaQualityBits.BadCommFailure:
                    text += "(Bad:Communication Failure";
                    break;
                case DaQualityBits.BadOutOfService:
                    text += "(Bad:Out of Service";
                    break;
                case DaQualityBits.BadWaitingForInitialData:
                    text += "(Bad:Waiting for Initial Data";
                    break;
                case DaQualityBits.Uncertain:
                    text += "(Uncertain";
                    break;
                case DaQualityBits.UncertainLastUsableValue:
                    text += "(Uncertain:Last Usable Value";
                    break;
                case DaQualityBits.UncertainSensorNotAccurate:
                    text += "(Uncertain:Sensor not Accurate";
                    break;
                case DaQualityBits.UncertainEuExceeded:
                    text += "(Uncertain:Engineering Unit exceeded";
                    break;
                case DaQualityBits.UncertainSubNormal:
                    text += "(Uncertain:Sub Normal";
                    break;
            }

            if (LimitBits != DaLimitBits.None)
            {
                text += String.Format(":[{0}]", LimitBits.ToString());
            }
            else
            {
                text += String.Format(":Not Limited");
            }

            if (VendorBits != 0)
            {
                text += String.Format(":{0,0:X})", VendorBits);
            }
            else
            {
                text += String.Format(")");
            }

            return text;
        }

        /// <summary>
        /// Determines whether the specified Object is equal to the current Quality
        /// </summary>
        public override bool Equals(object target)
        {
            if (target == null || target.GetType() != typeof(DaQuality)) return false;

            var quality = (DaQuality)target;

            if (QualityBits != quality.QualityBits) return false;
            if (LimitBits != quality.LimitBits) return false;
            if (VendorBits != quality.VendorBits) return false;

            return true;
        }

        /// <summary>
        /// Returns hash code for the current Quality.
        /// </summary>
        public override int GetHashCode()
        {
            return Code;
        }
        #endregion

        #region Private Members
        private DaQualityBits qualityBits_;
        private DaLimitBits limitBits_;
        private byte vendorBits_;
        #endregion

        /// <summary>
        /// A 'good' quality value.
        /// </summary>
        public static readonly DaQuality Good = new DaQuality(DaQualityBits.Good);

        /// <summary>
        /// An 'bad' quality value.
        /// </summary>
        public static readonly DaQuality Bad = new DaQuality(DaQualityBits.Bad);
    }

    /// <summary>
    /// A class that defines constants used by OPC applications.
    /// </summary>
    public class StatusCodes
    {
        /// <remarks/>
        public static bool Failed(int hresultcode)
        { return (hresultcode < 0); }

        /// <remarks/>
        public static bool Succeeded(int hresultcode)
        { return (hresultcode >= 0); }

        /// <summary>
        /// The operation completed successfully.
        /// </summary>
        public const int Good = 0x00000000;

        /// <summary>
        /// The operation failed.
        /// </summary>
        public const int Bad = 0x00000001;

        /// <summary>
        /// Requested operation is not implemented.
        /// </summary>
        public const int BadNotImplemented = -0x7FFFBFFF; // 0x80004001

        /// <summary>
        /// One or more arguments are invalid.
        /// </summary>
        public const int BadInvalidArgument = -0x7FF8FFA9; // 0x80070057

        /// <summary>
        /// An exception occured.
        /// </summary>
        public const int BadException = unchecked((int)0x80010105);

        /// <summary>
        /// The handle refers to a node that does not exist in the address space.
        /// </summary>
        public const int BadInvalidHandle = -0x3FFBFFFF; // 0xC0040001
        
        /// <summary>
        /// The Id of the property is invalid.
        /// </summary>
        public const int BadInvalidPropertyId = -0x3FFBFDFD; // 0xC0040203
    }

    /// <summary>
    /// Browse Mode enumerator
    /// </summary>
    public enum DaBrowseMode
    {
        /// <summary>
        /// Browse calls are handled in the generic server and return the item/branches that
        /// are defined in the cache.
        /// </summary>
        Generic = 0,
        /// <summary>
        /// Browse calls are handled in the customization plug-in and typically return all items that could
        /// be dynamically added to the cache.
        /// </summary>
        Custom = 1
    }

    /// <summary>Enumerator for browse mode selection</summary>
    public enum DaBrowseType
    {
        /// <summary>Select only branches</summary>
        Branch = 1,
        /// <summary>Select only leafs</summary>
        Leaf = 2,
        /// <summary>Select all branches and leafs</summary>
        Flat = 3,
    }

    /// <summary>
    /// Defines the way to move 'up' or 'down' or 'to' in a hierarchical address
    /// space.
    /// </summary>
    public enum DaBrowseDirection
    {
        /// <summary>move �up� in a hierarchical address space.</summary>
        Up = 1,
        /// <summary>move �down� in a hierarchical address space.</summary>
        Down = 2,
        /// <summary>move 'to' in a hierarchical address space.</summary>
        To = 3
    }

    /// <summary>
    /// A notification sent by the server when an event change occurs.
    /// </summary>
    public class AeConditionState
    {
        #region Public Members
        /// <summary>
        /// Event Category Identifier.
        /// </summary>
        public int ConditionId;
        /// <summary>
        /// Sub Condition Definition Identifier. It's 0 for Single State Conditions.
        /// </summary>
        public int SubConditionId;
        /// <summary>
        /// State of the condition.
        /// </summary>
        public bool ActiveState;
        /// <summary>
        /// Quality associated with the condition state.
        /// </summary>
        public DaQuality Quality = DaQuality.Bad;
        /// <summary>
        /// Number of attributes available.
        /// </summary>
        public int AttributeCount;
        /// <summary>
        /// Attribute values
        /// </summary>
        public object[] AttributeValues;
        /// <summary>
        /// The message string passed in by the client who last acknowledged this condition.
        /// </summary>
        public string Message;
        /// <summary>
        /// Event severity (1..1000).
        /// </summary>
        public int Severity;
        /// <summary>
        /// This flag indicates that the related condition requires acknowledgment of this event. The determination of those events which require acknowledgment is server specific. For example, transition into a LimitAlarm condition would likely require an acknowledgment, while the event notification of the resulting acknowledgment would likely not require an acknowledgment.
        /// </summary>
        public bool AckRequired;
        /// <summary>
        /// Time that the condition became active (for single-state conditions), or the time of the transition into the current sub-condition (for multi-state conditions). This time is used by the client when acknowledging the condition (see IOPCEventServer::AckCondition method).
        /// </summary>
        public DateTime TimeStamp;
        #endregion

        /// <summary>
        /// Initialize the condition state
        /// </summary>
        /// <param name="conditionId">Event Category Identifier</param>
        /// <param name="subConditionId">Sub Condition Definition Identifier. It's 0 for Single State Conditions.</param>
        /// <param name="activeState">State of the condition</param>
        /// <param name="quality">Quality associated with the condition state</param>
        /// <param name="attributeCount">Number of attributes available</param>
        /// <param name="attributeValues">Attribute values</param>
        public AeConditionState(int conditionId, int subConditionId, bool activeState, DaQuality quality, int attributeCount, object[] attributeValues)
        {
            ConditionId = conditionId;
            SubConditionId = subConditionId;
            ActiveState = activeState;
            Quality = quality;
            AttributeCount = attributeCount;
            AttributeValues = attributeValues;
            Message = null;
            Severity = 0;
            AckRequired = false;
            TimeStamp = DateTime.Now;
        }

        /// <summary>
        /// Initialize the condition state
        /// </summary>
        /// <param name="conditionId">Event Category Identifier</param>
        /// <param name="subConditionId">Sub Condition Definition Identifier. It's 0 for Single State Conditions.</param>
        /// <param name="activeState">State of the condition</param>
        /// <param name="quality">Quality associated with the condition state</param>
        /// <param name="attributeCount">Number of attributes available</param>
        /// <param name="attributeValues">Attribute values</param>
        /// <param name="message">The message string passed in by the client who last acknowledged this condition</param>
        /// <param name="severity">Event severity (1..1000)</param>
        /// <param name="ackRequired">This flag indicates that the related condition requires acknowledgment of this event</param>
        /// <param name="timeStamp">Time that the condition became active (for single-state conditions), or the time of the transition into the current sub-condition (for multi-state conditions)</param>
        public AeConditionState(int conditionId, int subConditionId, bool activeState, DaQuality quality, int attributeCount, object[] attributeValues, string message, int severity, bool ackRequired, DateTime timeStamp)
        {
            ConditionId = conditionId;
            SubConditionId = subConditionId;
            ActiveState = activeState;
            Quality = quality;
            AttributeCount = attributeCount;
            AttributeValues = attributeValues;
            Message = message;
            Severity = severity;
            AckRequired = ackRequired;
            TimeStamp = timeStamp;

        }
    }
    #endregion

    #region Callback delegates - DON'T USE DIRECTLY.

    #region Data Access Callback methods

    /// <summary>
    /// Generic server callback to add a new item to the server's address 
    /// space.
    /// </summary>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the item was successfully added to the cache.
    /// </returns>
    /// <param name="itemId">
    /// 	<para>Fully qualified item name. The OPC clients use this name to access the
    ///     item.</para>
    /// 	<para>
    ///         This is a fully qualified item identifier such as "device1.channel5.heater3".
    ///         The generic server part builds an appropriate hierarchical address space.<br/>
    ///         Note that the separator character ( . in the above sample identifier ) can be
    ///         changed with the
    ///         OnGetDaServerParameters function.
    ///     </para>
    /// </param>
    /// <param name="accessRights">
    /// Access rights as defined in the OPC specification: DaAccessRights.readable ,
    /// DaAccessRights.writable, DaAccessRights.readWritable
    /// </param>
    /// <param name="initValue">
    /// 	<para>Object with initial value and the item's canonical data type.</para>
    /// 	<para>A value must always be specified for the canonical data type to be
    ///     defined.</para>
    /// </param>
    /// <param name="active">Defines whether the cache for this item should be refreshed.</param>
    /// <param name="euType">Engineering unit types</param>
    /// <param name="minValue">Only used if euType is DaEuType.analog</param>
    /// <param name="maxValue">Only used if euType is DaEuType.analog</param>
    /// <param name="deviceItemHandle">
    /// Handle returned by the generic server executable to reference the item.<br/>
    /// This handle is passed as an item identifier in calls between the generic server and the
    /// customization plugin. It uses directly the device item from the generic server to allow fast access to the item data.
    /// </param>
    public delegate int AddItem(
                                string itemId,
                                DaAccessRights accessRights,
                                object initValue,
                                Boolean active,
                                DaEuType euType,
                                double minValue,
                                double maxValue,
                                out IntPtr deviceItemHandle
                            );

    /// <summary>
    /// Generic server callback to change an item value.
    /// </summary>
    /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.
    /// Returns StatusCodes.Good if the value was successfully written into the cache.</returns>
    /// <param name="deviceItemHandle">Generic Server device item handle</param>
    /// <param name="newValue">
    /// 	<para>Object with new item value.</para>
    /// 	<para>
    ///         The value must match the canonical data type of this item. The canonical date
    ///         type is the type of the value in the cache and is defined in the
    ///         <see cref="ClassicBaseNodeManager.AddItem">AddItem</see> or
    ///         <see cref="ClassicBaseNodeManager.AddAnalogItem">AddAnalogItem</see> method call. null
    ///         can be passed to change only the quality and timestamp.
    ///     </para>
    /// </param>
    /// <param name="quality">
    /// New quality of the item value. This is a short value (Int16) with a value of the
    /// OPCQuality enumerator.
    /// </param>
    /// <param name="timestamp">New timestamp of the new item value.</param>
    public delegate int SetItemValue(
                                IntPtr deviceItemHandle,
                                object newValue,
                                short quality,
                                DateTime timestamp);

    /// <summary>
    /// Generic server callback to remove an item from the server's address space.
    /// </summary>
    /// <param name="deviceItemHandle">Generic Server device item handle</param>
    /// <returns>Returns StatusCodes.Good if the item was successfully removed from the cache.</returns>
    public delegate int RemoveItem(IntPtr deviceItemHandle);

    /// <summary>
    /// Generic server callback to add a custom property to the server.
    /// </summary>
    /// <returns>
    /// Returns StatusCodes.Good if the property was successfully added to the generic
    /// server property list.
    /// </returns>
    /// <param name="propertyId">
    /// The property ID assigned to the property. 0000 to 4999 are reserved for OPC
    /// use.
    /// </param>
    /// <param name="description">A brief vendor supplied text description of the property.</param>
    /// <param name="valueType">The default value and type of the property.</param>
    public delegate int AddProperty(
                                int propertyId,
                                string description,
                                object valueType);

    /// <summary>
    /// Generic server callback to set the state of the OPC server.
    /// </summary>
    /// <param name="serverState">
    ///     The new state of the server as an
    ///     <see cref="ServerState">ServerState</see> enumerator value.
    /// </param>
    public delegate void SetServerState(
                                ServerState serverState);

    /// <summary>
    /// Generic server callback to get a list of items used at least by one client.
    /// </summary>
    /// <param name="numItemHandles">Number of defined item handles; -1 means that function is not supported by the generic server</param>
    /// <param name="deviceItemHandles">Array of Generic Server device item handles</param>
    public delegate void GetActiveItems(
                                out int numItemHandles,
                                out IntPtr[] deviceItemHandles);

    /// <summary>
    /// Generic server callback to get a list of clients connected to the server.
    /// </summary>
    /// <param name="numClientHandles">Number of connected clients</param>
    /// <param name="clientHandles">Array of Generic Server client handles</param>
    /// <param name="clientNames">Array of client names</param>
    public delegate void GetClients(out int numClientHandles, out IntPtr[] clientHandles, out String[] clientNames);

    /// <summary>
    /// Generic server callback to get a list of groups used by the specified client.
    /// </summary>
    /// <param name="clientHandle">The handle of the client</param>
    /// <param name="numGroupHandles">Number of connected clients</param>
    /// <param name="groupHandles">Array of Generic Server client handles</param>
    /// <param name="groupNames">Array of client names</param>
    public delegate void GetGroups(IntPtr clientHandle, out int numGroupHandles, out IntPtr[] groupHandles, out String[] groupNames);

    /// <summary>
    /// Generic server callback to get the group information for the specified group.
    /// </summary>
    /// <param name="groupHandle">The handle of the group</param>
    /// <param name="groupState">The group related information</param>
    public delegate void GetGroupState(IntPtr groupHandle, out DaGroupState groupState);

    /// <summary>
    /// Generic server callback to get the item information for all items added to the specified group.
    /// </summary>
    /// <param name="groupHandle">The handle of the group</param>
    /// <param name="numItemStates">Number of items in the group</param>
    /// <param name="itemStates">The item related information</param>
    public delegate void GetItemStates(IntPtr groupHandle, out int numItemStates, out DaItemState[] itemStates);

    /// <summary>
    /// Generic server callback to fire a 'Shutdown Request' to the subscribed clients.
    /// </summary>
    /// <param name="reason">The reason why the server wants to shutdown.</param>
    public delegate int FireShutdownRequest(String reason);

    #endregion

    #region Alarms&Events Callback methods
    /// <summary>Adds a Simple Event Category to the Alarms&amp;Event Space.</summary>
    /// <param name="categoryId">Identifier of the new Event Category. This Id must be unique within the Event Server.</param>
    /// <param name="categoryDescription">Description of the Event Category</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event category was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// The recommended Event Categories are described in Appendix B of the OPC Alarms&amp;Events
    /// Specification.
    /// </remarks>
    public delegate int AddSimpleEventCategory(
                                int categoryId,
                                String categoryDescription);

    /// <summary>Adds a Tracking Event Category to the Alarms&amp;Event Space.</summary>
    /// <param name="categoryId">Identifier of the new Event Category. This Id must be unique within the Event Server.</param>
    /// <param name="categoryDescription">Description of the Event Category</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event category was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// The recommended Event Categories are described in Appendix B of the OPC Alarms&amp;Events
    /// Specification.
    /// </remarks>
    public delegate int AddTrackingEventCategory(
                                int categoryId,
                                String categoryDescription);

    /// <summary>Adds a Condition Event Category to the Alarms&amp;Event Space.</summary>
    /// <param name="categoryId">Identifier of the new Event Category. This Id must be unique within the Event Server.</param>
    /// <param name="categoryDescription">Description of the Event Category</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event category was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// The recommended Event Categories are described in Appendix B of the OPC Alarms&amp;Events
    /// Specification.
    /// </remarks>
    public delegate int AddConditionEventCategory(
                                int categoryId,
                                string categoryDescription);

    /// <summary>Adds a vendor specific Attribute to the Alarms&amp;Event Category.</summary>
    /// <param name="categoryId">Identifier of an existing Event Category.</param>
    /// <param name="eventAttribute">Identifier of the new Event Attribute. This ID must be unique within the Event Server. </param>
    /// <param name="attributeDescription">Description of the Event Attribute.</param>
    /// <param name="dataType">Object identifying the data type of the event attribute.</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event attribute was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// Adds a vendor specific attribute to an Event Category. The recommended Event Attributes are described in Appendix C of the OPC OPC Alarms&amp;Events Specification. 
    /// It is not required to add the Attributes 'ACK COMMENT' and 'AREAS' because they are internally added when a new Category is created. 
    /// </remarks>
    public delegate int AddEventAttribute(
                                int categoryId,
                                int eventAttribute, 
                                String attributeDescription,
                                object dataType);

    /// <summary>Adds a Single State Condition Definition to the Alarms&amp;Event Category.</summary>
    /// <param name="categoryId">Identifier of an existing Event Category.</param>
    /// <param name="conditionId">Identifier of the new Event Condition Definition. This ID must be unique within the Alarms&amp;Events Server.</param>
    /// <param name="name">Text string with the name of the Event Condition Definition. This name must be unique within the larms&amp;Events Server. </param>
    /// <param name="condition">Text string with the condition represented by this definition.</param>
    /// <param name="severity">The urgency in the range of 1 ... 1000. This is a default value. </param>
    /// <param name="description">Text string with description. This text is used as message for the generated Events. This is a default value.</param>
    /// <param name="ackRequired">True if the Event Conditions which uses this Definition requires acknowledgement; otherwise False. This is a default value. </param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the Single State Condition Definition was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// Adds a definition to the Event Space which can be used by Single State Conditions. A Single State Condition has only one sub-state of interest, 
    /// where the condition is active or not. The values Severity, Description and AckRequired flag are the default values for those Conditions which uses this Definition. 
    /// The default values are used if no other values are specified with function ProcessConditionStateChages(). 
    /// </remarks>
    public delegate int AddSingleStateConditionDefinition(
                                int categoryId,
                                int conditionId,
                                string name,
                                string condition,
                                int severity,
                                string description,
                                bool ackRequired);

    /// <summary>Adds a Multi State Condition Definition to the Alarms&amp;Event Category.</summary>
    /// <param name="categoryId">Identifier of an existing Event Category.</param>
    /// <param name="conditionId">Identifier of the new Event Condition Definition. This ID must be unique within the Alarms&amp;Events Server.</param>
    /// <param name="name">Text string with the name of the Event Condition Definition. This name must be unique within the larms&amp;Events Server. </param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the Single State Condition Definition was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// Adds a definition to the Event Space which can be used by Multi State Conditions. A Multi State Condition has at least two sub-states of interest. 
    /// Sub-states are mutually exclusive, only one sub-state can be active. Use the function AddSubCondition() to add at least two Sub Condition 
    /// Definitions for the sub-states.  
    /// </remarks>
    public delegate int AddMultiStateConditionDefinition(
                                int categoryId,
                                int conditionId,
                                string name);

    /// <summary>Adds a Sub Condition Definition to an existing Multi State Condition Definition.</summary>
    /// <param name="conditionId">Identifier of an existing Event Condition Definition.</param>
    /// <param name="subConditionId">Identifier of the new Sub Condition Definition. This ID must be unique within the Event Condition Definition. 
    /// Value 0 is invalid because internally used.</param>
    /// <param name="name">Text string with the name of the Event Sub Condition Definition. This name must be unique within the larms&amp;Events Server. </param>
    /// <param name="condition">Text string with the condition represented by this definition.</param>
    /// <param name="severity">The urgency in the range of 1 ... 1000. This is a default value. </param>
    /// <param name="description">Text string with description. This text is used as message for the generated Events. This is a default value.</param>
    /// <param name="ackRequired">True if the Event Conditions which uses this Definition requires acknowledgement; otherwise False. This is a default value. </param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the Sub Condition Definition was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// This function must be called at least twice for each Multi State Condition Definition added with AddMultiStateConditionDef(). 
    /// A Multi State Condition has at least two sub-states of interest. Sub-states are mutually exclusive, only one sub-state can be active. The values Severity, 
    /// Description and AckRequired flag are the default values for those Conditions which uses this Definition. The default values are used if no other values are 
    /// specified with function ProcessConditionStateChages().  
    /// </remarks>
    public delegate int AddSubConditionDefinition(
                                int conditionId,
                                int subConditionId,
                                string name,
                                string condition,
                                int severity,
                                string description,
                                bool ackRequired);

    /// <summary>Adds an Area to the Alarms&amp;Event Process Area Space.</summary>
    /// <param name="parentAreaId">Identifier of an existing Event Area. Use AREAID_ROOT to add an Area to the Root Area.</param>
    /// <param name="areaId">Identifier of the new Event Area. This ID must be unique within the Alarms&amp;Events Server.</param>
    /// <param name="name">Name of the Event Area. Do not use default delimiter characters. See comment below.</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the Single State Condition Definition was successfully added to the event space.
    /// </returns>
    /// <remarks>
    /// The name parts of a fully qualified Area Name are separated by '.' characters. This default value can be changed by the static function 
    /// SetDelimiter(). Be sure that the specified Event Names doesn't include any default delimiter characters.   
    /// </remarks>
    public delegate int AddArea(
                                int parentAreaId,
                                int areaId,
                                string name);

    /// <summary>Adds an Event Source object to the Alarms&amp;Event Process Area Space.</summary>
    /// <param name="areaId">Identifier of an existing Event Area. Use AREAID_ROOT to add a top-level Event Source object.</param>
    /// <param name="sourceId">Identifier of the new Event Source. This ID must be unique within the Alarms&amp;Events Server.</param>
    /// <param name="sourceName">Name of the Event Source. If parameter 'multiSource' is true then this parameter specifies the fully qualified source name; otherwise only the partial source name.</param>
    /// <param name="multiSource">true if this Event Source object is shared by multiple Process Areas (see function AddExistingSource()); otherwise false.</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event source was successfully added to the process area space.
    /// </returns>
    /// <remarks>
    /// Each event generated by an Alarms&amp;Event Server has an associated source object. 
    /// </remarks>
    public delegate int AddSource(
                                int areaId,
                                int sourceId,
                                string sourceName,
                                Boolean multiSource);

    /// <summary>Adds an existing Event Source object to an additional Alarms&amp;Event Process Area Space.</summary>
    /// <param name="areaId">Identifier of an existing Event Area. Use AREAID_ROOT to add a top-level Event Source object.</param>
    /// <param name="sourceId">Identifier of an existing Event Source object. The object with this ID must be previously added to the Process Area Space by the 
    /// function AddSource() with an active 'multiSource' flag. </param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event source was successfully added to the process area space.
    /// </returns>
    /// <remarks>
    /// Use this function if an Event Source object should be a member of multiple Process Areas. 
    /// </remarks>
    public delegate int AddExistingSource(
                                int areaId,
                                int sourceId);

    /// <summary>Adds an Event Condition to the Alarms&amp;Event Process Area Space.</summary>
    /// <param name="sourceId">Identifier of an existing Event Source.</param>
    /// <param name="conditionDefinitionId">Identifier of an existing Event Condition Definition.</param>
    /// <param name="conditionId">Identifier of the new Event Condition. This ID must be unique within the Alarms&amp;Events Server.</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
    /// </returns>
    /// <remarks>
    /// An Event Condition associates a Condition Definition with an Event Source. If the state of a Condition changes then the server creates events. 
    /// </remarks>
    public delegate int AddCondition(
                                int sourceId,
                                int conditionDefinitionId,
                                int conditionId);

    /// <summary>Generates a Simple Event.</summary>
    /// <param name="categoryId">Identifier of an existing Event Category. Specifies the Event Category to which this Event belongs.</param>
    /// <param name="sourceId">Identifier of an existing Event Source. Specifies the object which generates the event notification.</param>
    /// <param name="message">Message text string which describes the Event.</param>
    /// <param name="severity">The urgency of the Event in the range of 1 ... 1000.</param>
    /// <param name="attributeCount">Number of attribute values specified in pvAttrValues. This number must be identical with the number of 
    /// attributes added to the specified Event Category. This parameter is only used for cross-check.</param>
    /// <param name="attributeValues">Array of attribute values. The order and the types must be identical with the attributes of the specified Event Category.</param> 
    /// <param name="timeStamp">Specifies the occurrence time of the event. If this parameter is a NULL pointer then the current time is used.</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
    /// </returns>
    /// <remarks>
    /// Use this function to generate a Single Event. A Single Events is not associated with an Event Condition. The generic server part uses the subscription 
    /// specific filters and forwards the Event to the subscriptions of all connected clients if the filters are passed.  
    /// </remarks>
    public delegate int ProcessSimpleEvent(
                                int categoryId,
                                int sourceId,
                                string message,
                                int severity,
                                int attributeCount,
                                object[] attributeValues,
                                DateTime timeStamp);

    /// <summary>Generates a Tracking Event.</summary>
    /// <param name="categoryId">Identifier of an existing Event Category. Specifies the Event Category to which this Event belongs.</param>
    /// <param name="sourceId">Identifier of an existing Event Source. Specifies the object which generates the event notification.</param>
    /// <param name="message">Message text string which describes the Event.</param>
    /// <param name="severity">The urgency of the Event in the range of 1 ... 1000.</param>
    /// <param name="actorId">Text string which identifies the OPC Client which initiated the action resulting the tracking-related Event.</param>
    /// <param name="attributeCount">Number of attribute values specified in pvAttrValues. This number must be identical with the number of 
    /// attributes added to the specified Event Category. This parameter is only used for cross-check.</param>
    /// <param name="attributeValues">Array of attribute values. The order and the types must be identical with the attributes of the specified Event Category.</param> 
    /// <param name="timeStamp">Specifies the occurrence time of the event. If this parameter is a NULL pointer then the current time is used.</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
    /// </returns>
    /// <remarks>
    /// Use this function to generate a Tracking Event. A Tracking Events is not associated with an Event Condition. The generic server part uses the 
    /// subscription specific filters and forwards the Event to the subscriptions of all connected clients if the filters are passed. 
    /// </remarks>
    public delegate int ProcessTrackingEvent(
                                int categoryId,
                                int sourceId,
                                string message,
                                int severity,
                                string actorId,
                                int attributeCount,
                                object[] attributeValues,
                                DateTime timeStamp);

    /// <summary>Changes the state of one or more Event Conditions.</summary>
    /// <param name="count">Number of Conditions to be changed.</param>
    /// <param name="conditionStateChanges">Array of class ConditionChangeStates with the new condition states.</param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
    /// </returns>
    /// <remarks>
    /// Changes the state of one or more Event Conditions. If the state of a condition changes then the generic server part creates Event instances .The generic server part uses the subscription specific 
    /// filters and forwards the generated Events to the subscriptions of all connected clients if the filters are passed. 
    /// </remarks>
    public delegate int ProcessConditionStateChanges(
                                int count,
                                AeConditionState[] conditionStateChanges);

    /// <summary>Internally acknowledgement of an Event Condition.</summary>
    /// <param name="conditionId">Identifier of an existing Event Condition.</param>
    /// <param name="comment">Text string with a comment. A NULL pointer means there is no comment. This parameter is optional. </param>
    /// <returns>
    ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
    ///     Returns StatusCodes.Good if the event condition was successfully acknowledged.
    /// </returns>
    /// <remarks>
    /// Internally acknowledgement of a Condition by the server if it's no longer required to be acknowledged by a client. 
    /// The generic function part calls the function OnAckNotification(). 
    /// </remarks>
    public delegate int AckCondition(
                                int conditionId,
                                string comment);
    #endregion

    #endregion

    /// <summary>
    ///  <para>OPC DA/AE Server Solution .NET API.</para>
    ///  <para>This class defines the generic server interface.</para>
    ///  <para>The ClassicBaseNodeManager class provides a set of generic server callback methods. 
    ///     These methods can be used to read information from the generic
    ///     server or change data in the generic server. They are always called by the
    ///     customization plugin.</para>
    ///  <para>It also defines classes and enumerators used in the data exchange with the
    ///     generic server and contains a standard implementation of the methods called by the
    ///     generic server, e.g. OnCreateServerItems.</para>
    ///  <para>The class ClassicNodeManager inherits from this class and defines method overloads
    ///     for the methods that need to be implemented for a specific application.</para>
    /// </summary>
    public class ClassicBaseNodeManager
    {
        #region Fields

        static private readonly Mutex mutexSetVal_ = new Mutex(false);

        static internal ClassicServerDefinition DaServer;
        static internal ClassicServerDefinition AeServer;

        #region Data Access Callback methods

        private static AddItem addItemCallback_;
        private static SetItemValue setItemValueCallback_;
        private static RemoveItem removeItemCallback_;
        private static AddProperty addPropertyCallback_;
        private static SetServerState setServerStateCallback_;
        private static GetActiveItems getActiveItemsCallback_;
        private static GetClients getClientsCallback_;
        private static GetGroups getGroupsCallback_;
        private static GetGroupState getGroupStateCallback_;
        private static GetItemStates getItemStatesCallback_;
        private static FireShutdownRequest fireShutdownRequestCallback_;

        #endregion

        #region Alarms&Events Callback methods

        private static AddSimpleEventCategory addSimpleEventCategory_;
        private static AddTrackingEventCategory addTrackingEventCategory_;
        private static AddConditionEventCategory addConditionEventCategory_;
        private static AddEventAttribute addEventAttribute_;
        private static AddSingleStateConditionDefinition addSingleStateConditionDefinition_;
        private static AddMultiStateConditionDefinition addMultiStateConditionDefinition_;
        private static AddSubConditionDefinition addSubConditionDefinition_;
        private static AddArea addArea_;
        private static AddSource addSource_;
        private static AddExistingSource addExistingSource_;
        private static AddCondition addCondition_;
        private static ProcessSimpleEvent processSimpleEvent_;
        private static ProcessTrackingEvent processTrackingEvent_;
        private static ProcessConditionStateChanges processConditionStateChanges_;
        private static AckCondition ackCondition_;

        #endregion

        #endregion

        #region General Methods (not related to an OPC specification)

        #region  .NET API Customization Callback Methods
        //---------------------------------------------------------------------
        //  .NET API Callback Methods 
        // (Called by the customization assembly)
        //---------------------------------------------------------------------

        /// <summary>
        /// 	<para>Generic server callback method.</para>
        /// 	<para>Set the OPC server state value, that is returned in client GetStatus
        ///     calls.</para>
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        /// <param name="serverState">
        ///     The new state of the server as an
        ///     <see cref="ServerState">ServerState</see> enumerator value.
        /// </param>
        public static int SetServerState(ServerState serverState)
        {
            if (setServerStateCallback_ != null)
            {
                setServerStateCallback_(serverState);
                return StatusCodes.Good;
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// Generic server callback to fire a 'Shutdown Request' to the subscribed clients.
        /// </summary>
        /// <param name="reason">The reason why the server wants to shutdown.</param>
        public static int FireShutdownRequest(String reason)
        {
            if (fireShutdownRequestCallback_ != null)
            {
                fireShutdownRequestCallback_(reason);
                return StatusCodes.Good;
            }
            return StatusCodes.BadNotImplemented;
        }

        #endregion

        #region  .NET API Generic Server Default Methods
        //---------------------------------------------------------------------
        //  .NET API Methods 
        // (Called by the generic server)
        //---------------------------------------------------------------------

        /// <summary>
        /// Gets the logging level to be used.
        /// </summary>
        /// <returns>
        ///     A LogLevel
        /// </returns>
        public virtual int OnGetLogLevel()
        {
            return (int)LogLevel.Info;
        }

        /// <summary>
        /// Gets the logging path to be used.
        /// </summary>
        /// <returns>
        ///     Path to be used for logging.
        /// </returns>
        public virtual string OnGetLogPath()
        {
            return "";
        }

        /// <summary>
        /// 	<para>
        ///         This method is called from the generic server at the startup; when the first
        ///         client connects or the service is started. All items supported by the server
        ///         need to be defined by calling the <see cref="AddItem">AddItem</see> or
        ///         <see cref="AddAnalogItem">AddAnalogItem</see> callback method for each item.
        ///     </para>
        /// 	<para>The Item IDs are fully qualified names ( e.g. Dev1.Chn5.Temp ).</para>
        /// 	<para>
        ///         If <see cref="DaBrowseMode">DaBrowseMode.Generic</see> is set the generic
        ///         server part creates an approriate hierarchical address space. The sample code
        ///         defines the application item handle as the buffer array index. This handle is
        ///         passed in the calls from the generic server to identify the item. It should
        ///         allow quick access to the item definition / buffer. The handle may be
        ///         implemented differently depending on the application.
        ///     </para>
        /// 	<para>The branch separator character used in the fully qualified item name must
        ///     match the separator character defined in the OnGetDaServerParameters method.</para>
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        public virtual int OnCreateServerItems()
        {
            // server has no items defined
            return StatusCodes.Good;
        }

		/// <summary>
		///     <para>This method is called from the generic server when a Startup is executed. It's the first DLL method called.</para>
		/// </summary>
		/// <param name="commandLine">String with the command line including parameters</param>
        public virtual void OnStartupSignal(string commandLine)
		{
			// no action required in the default implementation
		}

        /// <summary>
        /// 	<para>This method is called from the generic server when a Shutdown is
        ///     executed.</para>
        /// 	<para>To ensure proper process shutdown, any communication channels should be
        ///     closed and all threads terminated before this method returns.</para>
        /// </summary>
        public virtual void OnShutdownSignal()
        {
            // no action required in the default implementation
        }
        #endregion

        #region  .NET API Additional Methods
        //----------------------------------------------------------------------------
        //  .NET API additional Methods 
        // (Called by the generic server)
        //----------------------------------------------------------------------------

        /// <summary>
        /// This method is called when a client connects to the OPC server. 
        /// If the method returns an error code then the client connect is refused.
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.
        /// StatusCodes.Good allows the client to connect to the server.</returns>
        public virtual int OnClientConnect()
        {
            // client is allowed to connect to server
            return StatusCodes.Good;
        }

        /// <summary>
        /// This method is called when a client disconnects from the OPC server. 
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        public virtual int OnClientDisconnect()
        {
            return StatusCodes.Good;
        }

        #endregion

        #endregion

        #region Data Access related Methods
        
        #region  .NET API Customization Callback Methods
        //---------------------------------------------------------------------
        //  .NET API Callback Methods 
        // (Called by the customization assembly)
        //---------------------------------------------------------------------

        /// <summary>
        /// 	<para>
        ///         This function is called by the customization plugin and add an item to the
        ///         generic server cache. If
        ///         <see cref="DaBrowseMode">DaBrowseMode.Generic</see> is set the item is also
        ///         added to the the generic server internal browse hierarchy.
        ///     </para>
        /// 	<para>
        ///         The generic server sets the EU type to
        ///         <see cref="DaEuType">DaEuType.noEnum</see> and define an empty EUInfo.
        ///     </para>
        /// 	<para>This method is typically called during the execution of the
        ///     OnCreateServerItems method, but it can be called anytime an item needs to be added
        ///     to the generic server cache.</para>
        /// 	<para>Items that are added to the generic server cache can be accessed by OPC
        ///     clients.</para>
        /// </summary>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the item was successfully added to the cache.
        /// </returns>
        /// <param name="itemId">
        ///     Fully qualified item name. The OPC clients use this name to access the item.<br/>
        ///     This is a fully qualified item identifier such as "device1.channel5.heater3". The
        ///     generic server part builds an appropriate hierarchical address space.<br/>
        ///     Note that the separator character ( . in the above sample identifier ) can be
        ///     changed with the <see cref="OnGetDaServerParameters">OnGetDaServerParameters</see>
        ///     function.
        /// </param>
        /// <param name="accessRights">
        /// Access rights as defined in the OPC specification: DaAccessRights.readable ,
        /// DaAccessRights.writable, DaAccessRights.readWritable
        /// </param>
        /// <param name="initValue">
        /// Object with initial value and the item's canonical data type.<br/>
        /// A value must always be specified for the canonical data type to be defined.
        /// </param>
        /// <param name="deviceItemHandle">
        /// Handle returned by the generic server executable to reference the item.<br/>
        /// This handle is passed as an item identifier in calls between the generic server and the
        /// customization plugin. It uses directly the device item from the generic server to allow fast access to the item data.
        /// </param>
        public static int AddItem(string itemId, DaAccessRights accessRights, object initValue, out IntPtr deviceItemHandle)
        {
            deviceItemHandle = IntPtr.Zero;
            if (addItemCallback_ != null)
            {
                return addItemCallback_(itemId, accessRights, initValue, true, DaEuType.NoEnum, 0.0, 0.0, out deviceItemHandle);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// 	<para>
        ///         This function is called by the customization plugin and add an analog item to
        ///         the generic server cache. If
        ///         <see cref="DaBrowseMode">DaBrowseMode.Generic</see> is set the item is also
        ///         added to the the generic server internal browse hierarchy.
        ///     </para>
        /// 	<para>
        ///         The generic server sets the EU type to
        ///         <see cref="DaEuType">DaEuType.analog</see> and use the minValue and
        ///         maxValue for defining the correct EUInfo.
        ///     </para>
        /// 	<para>This method is typically called during the execution of the
        ///     OnCreateServerItems method, but it can be called anytime an item needs to be added
        ///     to the generic server cache.</para>
        /// 	<para>Items that are added to the generic server cache can be accessed by OPC
        ///     clients.</para>
        /// </summary>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the item was successfully added to the cache.
        /// </returns>
        /// <param name="itemId">
        ///     Fully qualified item name. The OPC clients use this name to access the item.<br/>
        ///     This is a fully qualified item identifier such as "device1.channel5.heater3". The
        ///     generic server part builds an appropriate hierarchical address space.<br/>
        ///     Note that the separator character ( . in the above sample identifier ) can be
        ///     changed with the <see cref="OnGetDaServerParameters">OnGetDaServerParameters</see>
        ///     function.
        /// </param>
        /// <param name="accessRights">
        /// Access rights as defined in the OPC specification: DaAccessRights.readable ,
        /// DaAccessRights.writable, DaAccessRights.readWritable
        /// </param>
        /// <param name="initValue">
        /// Object with initial value and the item's canonical data type.<br/>
        /// A value must always be specified for the canonical data type to be defined.
        /// </param>
        /// <param name="minValue">Only used if euType is <see cref="DaEuType">DaEuType.analog</see></param>
        /// <param name="maxValue">Only used if euType is <see cref="DaEuType">DaEuType.analog</see></param>
        /// <param name="deviceItemHandle">
        /// Handle returned by the generic server executable to reference the item.<br/>
        /// This handle is passed as an item identifier in calls between the generic server and the
        /// customization plugin. It uses directly the device item from the generic server to allow fast access to the item data.
        /// </param>
        public static int AddAnalogItem(string itemId, DaAccessRights accessRights, object initValue, double minValue, double maxValue, out IntPtr deviceItemHandle)
        {
            deviceItemHandle = IntPtr.Zero;
            if (addItemCallback_ != null)
            {
                return addItemCallback_(itemId, accessRights, initValue, true, DaEuType.Analog, minValue, maxValue, out deviceItemHandle);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// 	<para>
        ///         This function is called by the customization plugin and removes an item from
        ///         the generic server cache. If
        ///         <see cref="DaBrowseMode">DaBrowseMode.Generic</see> is set the item is also
        ///         removed from the the generic server internal browse hierarchy.
        ///     </para>
        /// 	<para>This method can be called anytime an item needs to be removed from the
        ///     generic server cache.</para>
        /// 	<para>Items that are removed from the generic server cache can no longer be
        ///     accessed by OPC clients.</para>
        /// </summary>
        /// <param name="deviceItemHandle">Generic Server device item handle</param>
        /// <returns>Returns StatusCodes.Good if the item was successfully removed from the cache.</returns>
        public static int RemoveItem(IntPtr deviceItemHandle)
        {
            if (removeItemCallback_ != null)
            {
                return removeItemCallback_(deviceItemHandle);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// Adds a custom specific property to the generic server list of item
        /// properties.
        /// </summary>
        /// <returns>
        /// Returns StatusCodes.Good if the property was successfully added to the generic
        /// server property list.
        /// </returns>
        /// <param name="propertyId">
        /// The property ID assigned to the property. 0000 to 4999 are reserved for OPC
        /// use.
        /// </param>
        /// <param name="description">A brief vendor supplied text description of the property.</param>
        /// <param name="valueType">The default value and type of the property.</param>
        public virtual int AddProperty(
                        int propertyId,
                        string description,
                        object valueType)
        {
            if (addPropertyCallback_ != null)
            {
                return addPropertyCallback_(propertyId, description, valueType);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// 	<para>Generic server callback method.</para>
        /// 	<para>Write an item value into the cache.</para>
        /// </summary>
        /// <returns>
        /// 	<para>
        ///         A <see cref="StatusCodes"/> code with the result of the operation.
        ///     </para>
        /// 	<para>Returns StatusCodes.Good if the value was successfully written into the
        ///     cache.</para>
        /// </returns>
        /// <param name="deviceItemHandle">Item handle as returned in the AddItem method call.</param>
        /// <param name="newValue">
        /// 	<para>Object with new item value.</para>
        /// 	<para>The value must match the canonical data type of this item. The canonical date
        ///     type is the type of the value in the cache and is defined in the AddItem method
        ///     call.<br/>
        ///     null can be passed to change only the quality and timestamp.</para>
        /// </param>
        /// <param name="quality">
        /// 	<para>New quality of the item value.</para>
        /// 	<para>This is a short value ( Int16) with a value of the OPCQuality
        ///     enumerator.</para>
        /// </param>
        /// <param name="timestamp">New timestamp of the new item value.</param>
        public static int SetItemValue(IntPtr deviceItemHandle, object newValue, short quality, DateTime timestamp)
        {
            int rtc;
            mutexSetVal_.WaitOne();
            try
            {
                if (setItemValueCallback_ == null)
                {
                    return StatusCodes.BadNotImplemented;
                }
                rtc = setItemValueCallback_(deviceItemHandle, newValue, quality, timestamp);
            }
            catch
            {
                rtc = StatusCodes.BadException;
            }
            mutexSetVal_.ReleaseMutex();
            return rtc;
        }

        /// <summary>
        /// Generic server callback to get a list of items used at least by one client.
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        /// <param name="numItemHandles">Number of defined item handles; -1 means that function is not supported by the generic server</param>
        /// <param name="deviceItemHandles">Array of Generic Server device item handles</param>
        public static int GetActiveItems(
                                    out int numItemHandles,
                                    out IntPtr[] deviceItemHandles)
        {
            if (getActiveItemsCallback_ != null)
            {
                getActiveItemsCallback_(out numItemHandles, out deviceItemHandles);
                return StatusCodes.Good;
            }
            numItemHandles = 0;
            deviceItemHandles = null;
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// Generic server callback to get a list of clients connected to the server.
        /// </summary>
        /// <param name="numClientHandles">Number of connected clients</param>
        /// <param name="clientHandles">Array of Generic Server client handles</param>
        /// <param name="clientNames">Array of client names</param>
        public static int GetClients(
                            out int numClientHandles,
                            out IntPtr[] clientHandles,
                            out String[] clientNames)
        {
            if (getClientsCallback_ != null)
            {
                getClientsCallback_(out numClientHandles, out clientHandles, out clientNames);
                return StatusCodes.Good;
            }
            numClientHandles = 0;
            clientHandles = null;
            clientNames = null;
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// Generic server callback to get a list of groups used by the specified client.
        /// </summary>
        /// <param name="clientHandle">The handle of the client</param>
        /// <param name="numGroupHandles">Number of connected clients</param>
        /// <param name="groupHandles">Array of Generic Server client handles</param>
        /// <param name="groupNames">Array of client names</param>
        public static int GetGroups(
                            IntPtr clientHandle,
                            out int numGroupHandles,
                            out IntPtr[] groupHandles,
                            out String[] groupNames)
        {
            if (getGroupsCallback_ != null)
            {
                getGroupsCallback_(clientHandle, out numGroupHandles, out groupHandles, out groupNames);
                return StatusCodes.Good;
            }
            numGroupHandles = 0;
            groupHandles = null;
            groupNames = null;
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// Generic server callback to get the group information for the specified group.
        /// </summary>
        /// <param name="groupHandle">The handle of the group</param>
        /// <param name="groupState">The group related information</param>
        public static int GetGroupState(IntPtr groupHandle, out DaGroupState groupState)
        {
            if (getGroupStateCallback_ != null)
            {
                getGroupStateCallback_(groupHandle, out groupState);
                return StatusCodes.Good;
            }
            groupState = new DaGroupState();
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>
        /// Generic server callback to get the item information for all items added to the specified group.
        /// </summary>
        /// <param name="groupHandle">The handle of the group</param>
        /// <param name="numItemStates">Number of items in the group</param>
        /// <param name="itemStates">The item related information</param>
        public static int GetItemStates(IntPtr groupHandle, out int numItemStates, out DaItemState[] itemStates)
        {
            if (getItemStatesCallback_ != null)
            {
                getItemStatesCallback_(groupHandle, out numItemStates, out itemStates);
                return StatusCodes.Good;
            }
            numItemStates = 0;
            itemStates = null;
            return StatusCodes.BadNotImplemented;
        }
        #endregion

        #region  .NET API Generic Server Default Methods
        //---------------------------------------------------------------------
        //  .NET API Methods 
        // (Called by the generic server)
        //---------------------------------------------------------------------

        /// <summary>
        /// 	<para>This method is called from the generic server at startup. It passes the
        ///  callback methods supported by the generic server. These callback methods can be
        ///  called anytime to exchange data with the generic server.</para>
        /// 	<para>The DefineCallbacks method need not be overloaded or changed. The default
        ///  implementation stores the delegates for later callbacks.</para>
        /// </summary>
        /// <param name="addItem">Add an item to the server's address space</param>
        /// <param name="removeItem">Removes an item from the server's adress space</param>
        /// <param name="addProperty">Add a custom property to the server</param>
        /// <param name="setItemValue">Writes a new item value into the server's cache</param>
        /// <param name="setServerState">
        /// 	<para>Set the OPC server state value, that is returned in client GetStatus
        ///     calls.</para>
        /// </param>
        /// <param name="getActiveItems">Get a list of items used at least by one client</param>
        /// <param name="getClients">Get a list of clients connected to the server</param>
        /// <param name="getGroups">Get a list of groups</param>
        /// <param name="getGroupState">Get information about a group</param>
        /// <param name="getItemStates">Get information about all items within a group</param>
        /// <param name="fireShutdownRequest">Fires a shutdown request to all connected clients.</param>
        public void OnDefineDaCallbacks(AddItem addItem, RemoveItem removeItem, AddProperty addProperty, SetItemValue setItemValue, SetServerState setServerState, GetActiveItems getActiveItems, GetClients getClients, GetGroups getGroups, GetGroupState getGroupState, GetItemStates getItemStates, FireShutdownRequest fireShutdownRequest)
        {
            addItemCallback_ = addItem;
            addPropertyCallback_ = addProperty;
            setItemValueCallback_ = setItemValue;
            removeItemCallback_ = removeItem;
            setServerStateCallback_ = setServerState;
            getActiveItemsCallback_ = getActiveItems;
            getClientsCallback_ = getClients;
            getGroupsCallback_ = getGroups;
            getGroupStateCallback_ = getGroupState;
            getItemStatesCallback_ = getItemStates;
            fireShutdownRequestCallback_ = fireShutdownRequest;
        }


        /// <summary>
        /// 	<para>This method is called from the generic server at startup for normal operation or for registration. It provides server registry information for this
        /// application required for DCOM registration. The generic server registers the OPC server accordingly.</para>
        /// 	<para>The default implementation in <em>ClassicBaseNodeManager</em> returns an empty configuration. The method can be replaced by overriding it in the
        /// <em>ClassicNodeManager</em>.</para>
        /// </summary>
        /// <remarks>
        /// 	<para>The default implementation in ClassicBaseNodeManager.cs returns an empty configuration. The CLSID definitions need to be unique and can be created with the
        /// Visual Studio <em>Create GUID</em> tool.</para>
        /// </remarks>
        /// <returns>Definition structure.</returns>
        /// <example>
        /// 	<para>
        /// 		<font color="blue" size="2" face="Consolas">
        /// 			<font color="blue" size="2" face="Consolas">
        /// 				<font color="blue" size="2" face="Consolas">public</font>
        /// 			</font>
        /// 		</font>
        /// 		<font color="blue" size="2" face="Consolas">
        /// 			<font color="blue" size="2" face="Consolas">
        /// 				<font color="blue" size="2" face="Consolas">override</font>
        /// 			</font>
        /// 		</font>
        /// 		<font color="#2B91AF" size="2" face="Consolas">
        /// 			<font color="#2B91AF" size="2" face="Consolas">
        /// 				<font color="#2B91AF" size="2" face="Consolas">ClassicServerRegistryInfo</font>
        /// 			</font>
        /// 		</font>
        /// 		<font size="2" face="Consolas">
        /// 			<font size="2" face="Consolas">OnGetDaServerDefinition()</font>
        /// 			<br/>
        /// 		</font>
        /// 		<font size="2" face="Consolas">
        /// 			<font size="2" face="Consolas">{</font>
        /// 		</font>
        /// 	</para>
        /// 	<blockquote style="MARGIN-RIGHT: 0px" dir="ltr">
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer =</font>
        /// 					</font>
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">new</font>
        /// 						</font>
        /// 					</font>
        /// 					<font color="#2B91AF" size="2" face="Consolas">
        /// 						<font color="#2B91AF" size="2" face="Consolas">
        /// 							<font color="#2B91AF" size="2" face="Consolas">ClassicServerRegistryInfo</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">();</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#region</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">General
        ///     Settings</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer.ClsIdApp
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"{9236F2A6-96EA-4D44-8C42-3A6DDA061BC6}"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer.CompanyName
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware GmbH"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#endregion</font>
        /// 						</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#region</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DA Server
        ///     registry definitions</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer.ClsIdServer
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"{9B59C648-8FA5-4BBA-9686-7CDB5041C456}"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer.PrgIdServer
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware.DaSample"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer.PrgIdCurrServer
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware.DaSample.20"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer.ServerName
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware OPC DA Sample Server"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer.CurrServerName
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware OPC DA Sample Server V2.0"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#endregion</font>
        /// 						</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">return</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DaServer;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 	</blockquote>
        /// 	<para>
        /// 		<font size="2" face="Consolas">
        /// 			<font size="2" face="Consolas">}</font>
        /// 		</font>
        /// 	</para>
        /// </example>
        public virtual ClassicServerDefinition OnGetDaServerDefinition()
        {
            DaServer = new ClassicServerDefinition();

            return DaServer;
        }

        /// <summary>
        /// 	<para>This method is called from the generic server at startup; when the first
        ///     client connects or the service is started.</para>
        /// 	<para>It defines the application specific server parameters and operating modes.
        ///     The default implementation in <em>ClassicBaseNodeManager</em> initializes default values</para>
        /// 	<para>The default method can be replaced by overriding it in the
        ///     <em>ClassicNodeManager</em>.</para>
        /// </summary>
        /// <param name="updatePeriod">
        /// 	<para>This interval in ms is used by the generic server as the fastest possible
        ///     client update rate and also uses this definition when determining the refresh need
        ///     if no client defined a sampling rate for the item. The default value is 200.</para>
        /// </param>
        /// <param name="branchDelimiter">
        /// 	<para>Character is used as the branch/item separator character in fully qualified
        ///     item names. It is typically '.' or '/'.</para>
        /// 	<para>This character must match the character used in the fully qualified item IDs
        ///     specified in the AddItems method call.</para>
        /// </param>
        /// <param name="browseMode">
        /// 	<para>Defines how client browse calls are handled.</para>
        /// 	<para>0 (Generic) : all browse calls are handled in the generic server according
        ///     the items defined in the server cache.<br/>
        ///     1 (Custom) : all client browse calls are handled in the customizatiopn plug-in and
        ///     typically return the items that are or could be dynamically added to the server
        ///     cache. The default value is "Generic".</para>
        /// </param>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.
        /// Always returns StatusCodes.Good</returns>
        public virtual int OnGetDaServerParameters(out int updatePeriod, out char branchDelimiter, out DaBrowseMode browseMode)
        {
            // Default Values
            branchDelimiter = '.';
            browseMode = DaBrowseMode.Generic;            // browse the generic server address space
            updatePeriod = 100;

            return StatusCodes.Good;
        }

        /// <summary>
        ///  	<para>This method is called from the generic server at startup; when the first
        ///     client connects or the service is started.</para>
        /// 	<para>It defines the application specific optimization parameters.
        ///     The default implementation in <em>ClassicBaseNodeManager</em> initializes default values.</para>
        /// </summary>
        /// <param name="useOnItemRequest">Specifiy whether OnItemRequest is called by the generic server; default is true</param>
        /// <param name="useOnRefreshItems">Specifiy whether OnRefreshItems is called by the generic server; default is true</param>
        /// <param name="useOnAddItem">Specifiy whether OnAddItem is called by the generic server; default is false</param>
        /// <param name="useOnRemoveItem">Specifiy whether OnRemoveItem is called by the generic server; default is false</param>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.
        /// Always returns StatusCodes.Good</returns>
        public virtual int OnGetDaOptimizationParameters(out bool useOnItemRequest, out bool useOnRefreshItems, out bool useOnAddItem, out bool useOnRemoveItem)
        {
            useOnItemRequest = true;
            useOnRefreshItems = true;
            useOnAddItem = false;
            useOnRemoveItem = false;

            return StatusCodes.Good;
        }

        /// <summary>
        /// Query the properties defined for the specified item
        /// </summary>
        /// <param name="deviceItemHandle">Application item handle</param>
        /// <param name="noProp">Number of properties returned</param>
        /// <param name="iDs">Array with the the property ID number</param>
        /// <returns>A <see cref="StatusCodes" /> code with the result of the operation. 
        ///  StatusCodes.Bad if the item has no custom properties.</returns>
        public virtual int OnQueryProperties(
            IntPtr deviceItemHandle,
            out int noProp,
            out int[] iDs)
        {
            // item has no custom properties
            noProp = 0;
            iDs = null;
            return StatusCodes.Bad;
        }

        /// <summary>
        /// Returns the values of the requested custom properties of the requested item. This
        /// method is not called for the OPC standard properties 1..8. These are handled in the
        /// generic server.
        /// </summary>
        /// <returns>HRESULT success/error code. Bad if the item has no custom properties.</returns>
        /// <param name="deviceItemHandle">Generic Server device item handle</param>
        /// <param name="propertyId">ID of the property</param>
        /// <param name="propertyValue">Property value</param>
        public virtual int OnGetPropertyValue(IntPtr deviceItemHandle, int propertyId, out object propertyValue)
        {
            // Item property is not available
            propertyValue = null;
            return StatusCodes.BadInvalidPropertyId;
        }

        /// <summary>
        /// 	<para>This method is called when a client executes a 'write' server call. The items
        ///     specified in the DaDeviceItemValue array need to be written to the device.</para>
        /// 	<para>The cache is updated in the generic server after returning from the
        ///     customization WiteItems method. Items with write error are not updated in the
        ///     cache.</para>
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        /// <param name="values">Object with handle, value, quality, timestamp</param>
        /// <param name="errors">Array with HRESULT success/error codes on return.</param>
        public virtual int OnWriteItems(DaDeviceItemValue[] values, out int[] errors)
        {
            // no item could be written
            errors = new int[values.Length];    // result array
            for (int i = 0; i < values.Length; ++i)   // init to Good
                errors[i] = StatusCodes.BadInvalidHandle;
            return StatusCodes.Bad;
        }

        /// <summary>
        ///  <para>Refresh the items listed in the deviceItemHandles array in the cache.</para>
        ///  <para>This method is called when a client executes a read from device. The device
        ///     read is called with all client requested items.</para>
        /// </summary>
        /// <param name="deviceItemHandles">Array of Generic Server device item handles</param>
        /// <returns>A <see cref="StatusCodes" /> code with the result of the operation.</returns>
        public virtual int OnRefreshItems(IntPtr[] deviceItemHandles)
        {
            // no items handled in this default implementation
            return StatusCodes.Good;
        }

        /// <summary>
        ///  <para>The item referenced by deviceItemHandle was added to a group or gets used for item based read/write.</para>
        ///  <para>This method is called when a client adds the items to a group or use item based read/write functions.</para>
        /// </summary>
        /// <param name="deviceItemHandle">Generic Server device item handle</param>
        /// <returns>A <see cref="StatusCodes" /> code with the result of the operation.</returns>
        public virtual int OnAddItem(IntPtr deviceItemHandle)
        {
            return StatusCodes.Good;
        }

        /// <summary>
        ///  <para>The item referenced by deviceItemHandle is no longer used by clients.</para>
        ///  <para>This method is called when a client removes items from a group or no longer use the items in item based read/write functions. 
        ///     Only items are listed which are no longer used by at least one client.</para>
        /// </summary>
        /// <param name="deviceItemHandle">Generic Server device item of the item that is no longer need to be updated.</param>
        /// <returns>A <see cref="StatusCodes" /> code with the result of the operation.</returns>
        public virtual int OnRemoveItem(IntPtr deviceItemHandle)
        {
            return StatusCodes.Good;
        }
        #endregion

        #region  .NET API Dynamic Address Space handling Methods
        //----------------------------------------------------------------------------
        //  .NET API Dynamic address space Handling Methods 
        // (Called by the generic server)
        //----------------------------------------------------------------------------

        /// <summary>
        /// 	<para>This method is called when the client accesses items that do not yet exist in
        ///     the server's cache.</para>
        /// 	<para>OPC DA 2.00 clients typically first call AddItems() or ValidateItems(). OPC
        ///     DA 3.00 client may access items directly using the ItemIO read/write functions.
        ///     Within this function it is possible to:</para>
        /// 	<para class="xmldocbulletlist"></para>
        /// 	<list type="bullet">
        /// 		<item>add the item to the servers real address space and return
        ///         StatusCodes.Good. For each item to be added the callback method 'AddItem' has
        ///         to be called.</item>
        /// 		<item>return StatusCodes.Bad</item>
        /// 	</list>
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        /// <param name="fullItemId">Name of the item which does not exist in the server's cache</param>
        public virtual int OnItemRequest(string fullItemId)
        {
            // no valid item in this default implementation
            return StatusCodes.Bad;
        }

        /// <summary>
        /// 	<para>Custom mode browse handling. Provides a way to move �up� or �down� or 'to' in
        ///     a hierarchical space.</para>
        /// 	<para>
        ///         Called only from the generic server when
        ///         <see cref="DaBrowseMode">DaBrowseMode.Custom</see> is configured.
        ///     </para>
        /// 	<para>Change the current browse branch to the specified branch in virtual address
        ///     space. This method has to be implemented according the OPC DA specification. The
        ///     generic server calls this fuction for OPC DA 2.05a and OPC DA 3.00 client calls.
        ///     The differences between the specifications is handled within the generic server
        ///     part. Please note that flat address space is not supported.</para>
        /// </summary>
        /// <returns>
        /// 	<para>
        ///         A <see cref="StatusCodes">StatusCodes</see> code with the result of the
        ///         operation.
        ///     </para>
        /// 	<para>
        /// 		<list type="table">
        /// 			<item>
        /// 				<term>
        /// 					<para><font size="1">E_FAIL<br/>
        ///                     E_OUTOFMEMORY<br/>
        ///                     BadInvalidArgument<br/>
        ///                     E_INVALIDFILTER<br/>
        ///                     Good<br/>
        ///                     Bad</font></para>
        /// 				</term>
        /// 				<description><font size="1">The operation failed.<br/>
        ///                 Not enough memory<br/>
        ///                 An argument to the function was invalid.<br/>
        ///                 The filter string was not valid<br/>
        ///                 The operation succeeded.<br/>
        ///                 No items meet the filter criteria.</font></description>
        /// 			</item>
        /// 		</list>
        /// 	</para>
        /// </returns>
        /// <remarks>
        /// 	<para>An error is returned if the passed string does not represent a
        ///     �branch�.</para>
        /// 	<para>Moving Up from the �root� will return E_FAIL.</para>
        /// 	<para>Note DaBrowseDirection.To is new for DA version 2.0. Clients should be
        ///     prepared to handle BadInvalidArgument if they pass this to a DA 1.0 server.</para>
        /// </remarks>
        /// <param name="browseDirection">DaBrowseDirection.To, DaBrowseDirection.Up or DaBrowseDirection.Down</param>
        /// <param name="position">
        /// 	<para>New absolute or relative branch. If DaBrowseDirection.Down the branch where
        ///     to change and if DaBrowseDirection.To the fully qualified name where to change or
        ///     NULL to go to the 'root'.</para>
        /// 	<para>
        ///         For DaBrowseDirection.Down, the name of the branch to move into. This would
        ///         be one of the strings returned from
        ///         <see cref="OnBrowseItemIds">OnBrowseItemIDs</see>. E.g. REACTOR10
        ///     </para>
        /// 	<para>For DaBrowseDirection.Up this parameter is ignored and should point to a
        ///     NULL string.</para>
        /// 	<para>For DaBrowseDirection.To a fully qualified name (e.g. as returned from
        ///     GetItemID) or a pointer to a NULL string to go to the 'root'. E.g.
        ///     AREA1.REACTOR10.TIC1001</para>
        /// </param>
        /// <param name="actualPosition">Actual position in the address tree for the calling client.</param>
        public virtual int OnBrowseChangePosition(DaBrowseDirection browseDirection, string position, ref string actualPosition)
        {
            // not supported in this default implementation
            return StatusCodes.BadInvalidArgument;
        }

        /// <summary>
        /// 	<para>Custom mode browse handling.</para>
        /// 	<para>
        ///         Called only from the generic server when
        ///         <see cref="DaBrowseMode">DaBrowseMode.Custom</see> is configured.
        ///     </para>
        /// 	<para>
        ///         This method browses the items in the current branch of the virtual address
        ///         space. The position from the which the browse is done can be set via
        ///         <see cref="OnBrowseChangePosition">OnBrowseChangePosition</see>. The generic
        ///         server calls this fuction for OPC DA 2.05a and OPC DA 3.00 client calls. The
        ///         differences between the specifications is handled within the generic server
        ///         part. Please note that flat address space is not supported.
        ///     </para>
        /// </summary>
        /// <returns>
        /// 	<para>
        ///         A <see cref="StatusCodes"/> code with the result of the operation.
        ///     </para>
        /// 	<para>
        /// 		<list type="table">
        /// 			<item>
        /// 				<term>
        /// 					<para><font size="1">E_FAIL<br/>
        ///                     E_OUTOFMEMORY<br/>
        ///                     BadInvalidArgument<br/>
        ///                     E_INVALIDFILTER<br/>
        ///                     Good<br/>
        ///                     Bad</font></para>
        /// 				</term>
        /// 				<description><font size="1">The operation failed.<br/>
        ///                 Not enough memory<br/>
        ///                 An argument to the function was invalid.<br/>
        ///                 The filter string was not valid<br/>
        ///                 The operation succeeded.<br/>
        ///                 No items meet the filter criteria.</font></description>
        /// 			</item>
        /// 		</list>
        /// 	</para>
        /// </returns>
        /// <remarks>
        /// 	<para>The returned enumerator may have nothing to enumerate if no ItemIDs satisfied
        ///     the filter constraints. The strings returned by the enumerator represent the
        ///     BRANCHs and LEAFS contained in the current level. They do NOT include any
        ///     delimiters or �parent� names.</para>
        /// 	<para>Whenever possible the server should return strings which can be passed
        ///     directly to AddItems. However, it is allowed for the Server to return a �hint�
        ///     string rather than an actual legal Item ID. For example a PLC with 32000 registers
        ///     could return a single string of �0 to 31999� rather than return 32,000 individual
        ///     strings from the enumerator. For this reason (as well as the fact that browser
        ///     support is optional) clients should always be prepared to allow manual entry of
        ///     ITEM ID strings. In the case of �hint� strings, there is no indication given as to
        ///     whether the returned string will be acceptable by AddItem or ValidateItem.</para>
        /// 	<para>Clients are allowed to get and hold Enumerators for more than one �browse
        ///     position� at a time.</para>
        /// 	<para>Changing the browse position will not affect any String Enumerator the client
        ///     already has.</para>
        /// 	<para>The client must Release each Enumerator when he is done with it.</para>
        /// </remarks>
        /// <param name="actualPosition">
        /// Position in the server address space (ex. "INTERBUS1.DIGIN") for the calling
        /// client
        /// </param>
        /// <param name="browseFilterType">
        /// 	<para>
        ///         Branch/Leaf filter: <see cref="DaBrowseType">DaBrowseType</see>
        /// 	</para>
        /// 	<para>Branch: returns only items that have children<br/>
        ///     Leaf: returns only items that don't have children<br/>
        ///     Flat: returns everything at and below this level including all children of children
        ///     - basically 'pretends' that the address space is actually FLAT</para>
        /// </param>
        /// <param name="filterCriteria">name pattern match expression, e.g. "*"</param>
        /// <param name="dataTypeFilter">
        /// Filter the returned list based on the available datatypes (those that would
        /// succeed if passed to AddItem). System.Void indicates no filtering.
        /// </param>
        /// <param name="accessRightsFilter">
        ///     Filter based on the AccessRights bit mask
        ///     <see cref="DaAccessRights">DaAccessRights</see>.
        /// </param>
        /// <param name="noItems">Number of items returned</param>
        /// <param name="itemIDs">Items meeting the browse criteria.</param>
        public virtual int OnBrowseItemIds(
                            string actualPosition,
                            DaBrowseType browseFilterType,
                            string filterCriteria,
                            Type dataTypeFilter,
                            DaAccessRights accessRightsFilter,
                            out int noItems,
                            out string[] itemIDs)
        {
            // not supported in this default implementation
            noItems = 0;
            itemIDs = null;
            return StatusCodes.BadInvalidArgument;
        }

        /// <summary>
        /// 	<para>Custom mode browse handling.</para>
        /// 	<para>
        ///         Called only from the generic server when
        ///         <see cref="DaBrowseMode">DaBrowseMode.Custom</see> is configured.
        ///     </para>
        /// 	<para>This method returns the fully qualified name of the specified item in the
        ///     current branch in the virtual address space. This name is used to add the item to
        ///     the real address space. The generic server calls this fuction for OPC DA 2.05a and
        ///     OPC DA 3.00 client calls. The differences between the specifications is handled
        ///     within the generic server part. Please note that flat address space is not
        ///     supported.</para>
        /// </summary>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        /// <remarks>
        /// 	<para>Provides a way to assemble a �fully qualified� ITEM ID in a hierarchical
        ///     space. This is required since the browsing functions return only the components or
        ///     tokens which make up an ITEMID and do NOT return the delimiters used to separate
        ///     those tokens. Also, at each point one is browsing just the names �below� the
        ///     current node (e.g. the �units� in a �cell�).</para>
        /// 	<para>A client would browse down from AREA1 to REACTOR10 to TIC1001 to
        ///     CURRENT_VALUE. As noted earlier the client sees only the components, not the
        ///     delimiters which are likely to be very server specific. The function rebuilds the
        ///     fully qualified name including the vendor specific delimiters for use by ADDITEMs.
        ///     An extreme example might be a server that returns:
        ///     <a href="file:////AREA1:REACTOR10.TIC1001[CURRENT_VALUE">\\AREA1:REACTOR10.TIC1001[CURRENT_VALUE</a>]</para>
        /// 	<para>It is also possible that a server could support hierarchical browsing of an
        ///     address space that contains globally unique tags. For example in the case above,
        ///     the tag TIC1001.CURRENT_VALUE might still be globally unique and might therefore be
        ///     acceptable to AddItem. However the expected behavior is that (a) GetItemID will
        ///     always return the fully qualified name (AREA1.REACTOR10.TIC1001.CURRENT_VALUE) and
        ///     that (b) that the server will always accept the fully qualified name in AddItems
        ///     (even if it does not require it).</para>
        /// 	<para>It is valid to form an ItemID that represents a BRANCH (e.g.
        ///     AREA1.REACTOR10). This could happen if you pass a BRANCH (AREA1) rather than a LEAF
        ///     (CURRENT_VALUE). The resulting string might fail if passed to AddItem but could be
        ///     passed to ChangeBrowsePosition using OPC_BROWSE_TO.</para>
        /// 	<para>The client must free the returned string.</para>
        /// 	<para>ItemID is the unique �key� to the data, it is considered the �what� or
        ///     �where� that allows the server to connect to the data source.</para>
        /// </remarks>
        /// <param name="actualPosition">Fully qualified name of the current branch</param>
        /// <param name="itemName">
        /// The name of a BRANCH or LEAF at the current level. Or a pointer to a NULL string.
        /// Passing in a NULL string results in a return string which represents the current
        /// position in the hierarchy.
        /// </param>
        /// <param name="fullItemId">
        /// Fully qualified name if the item. This name is used to access the item or add it
        /// to a group.
        /// </param>
        public virtual int OnBrowseGetFullItemId(string actualPosition, string itemName, out string fullItemId)
        {
            // not supported in this default implementation
            fullItemId = null;
            return StatusCodes.BadInvalidArgument;
        }
        #endregion

        #endregion

        #region Alarms&Events related Methods

        #region  .NET API Customization Callback Methods
        //---------------------------------------------------------------------
        //  .NET API Callback Methods 
        // (Called by the customization assembly)
        //---------------------------------------------------------------------


        /// <summary>Adds a Simple Event Category to the Alarms&amp;Event Space.</summary>
        /// <param name="categoryId">Identifier of the new Event Category. This Id must be unique within the Event Server.</param>
        /// <param name="categoryDescription">Description of the Event Category</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event category was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// The recommended Event Categories are described in Appendix B of the OPC Alarms&amp;Events
        /// Specification.
        /// </remarks>
        public static int AddSimpleEventCategory(
                                    int categoryId,
                                    String categoryDescription)
        {
            if (addSimpleEventCategory_ != null)
            {
                return addSimpleEventCategory_(categoryId, categoryDescription);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds a Tracking Event Category to the Alarms&amp;Event Space.</summary>
        /// <param name="categoryId">Identifier of the new Event Category. This Id must be unique within the Event Server.</param>
        /// <param name="categoryDescription">Description of the Event Category</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event category was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// The recommended Event Categories are described in Appendix B of the OPC Alarms&amp;Events
        /// Specification.
        /// </remarks>
        public static int AddTrackingEventCategory(
                                    int categoryId,
                                    String categoryDescription)
        {
            if (addTrackingEventCategory_ != null)
            {
                return addTrackingEventCategory_(categoryId, categoryDescription);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds a Condition Event Category to the Alarms&amp;Event Space.</summary>
        /// <param name="categoryId">Identifier of the new Event Category. This Id must be unique within the Event Server.</param>
        /// <param name="categoryDescription">Description of the Event Category</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event category was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// The recommended Event Categories are described in Appendix B of the OPC Alarms&amp;Events
        /// Specification.
        /// </remarks>
        public static int AddConditionEventCategory(
                                    int categoryId,
                                    String categoryDescription)
        {
            if (addConditionEventCategory_ != null)
            {
                return addConditionEventCategory_(categoryId, categoryDescription);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds a vendor specific Attribute to the Alarms&amp;Event Category.</summary>
        /// <param name="categoryId">Identifier of an existing Event Category.</param>
        /// <param name="eventAttribute">Identifier of the new Event Attribute. This ID must be unique within the Event Category. </param>
        /// <param name="attributeDescription">Description of the Event Attribute.</param>
        /// <param name="dataType">Object identifying the data type of the event attribute.</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event attribute was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// Adds a vendor specific attribute to an Event Category. The recommended Event Attributes are described in Appendix C of the OPC OPC Alarms&amp;Events Specification. 
        /// It is not required to add the Attributes 'ACK COMMENT' and 'AREAS' because they are internally added when a new Category is created. 
        /// </remarks>
        public static int AddEventAttribute(
                                    int categoryId,
                                    int eventAttribute,
                                    String attributeDescription,
                                    object dataType)
        {
            if (addEventAttribute_ != null)
            {
                return addEventAttribute_(categoryId, eventAttribute, attributeDescription, dataType);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds a Single State Condition Definition to the Alarms&amp;Event Category.</summary>
        /// <param name="categoryId">Identifier of an existing Event Category.</param>
        /// <param name="conditionId">Identifier of the new Event Condition Definition. This ID must be unique within the Alarms&amp;Events Server.</param>
        /// <param name="name">Text string with the name of the Event Condition Definition. This name must be unique within the larms&amp;Events Server. </param>
        /// <param name="condition">Text string with the condition represented by this definition.</param>
        /// <param name="severity">The urgency in the range of 1 ... 1000. This is a default value. </param>
        /// <param name="description">Text string with description. This text is used as message for the generated Events. This is a default value.</param>
        /// <param name="ackRequired">True if the Event Conditions which uses this Definition requires acknowledgement; otherwise False. This is a default value. </param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the Single State Condition Definition was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// Adds a definition to the Event Space which can be used by Single State Conditions. A Single State Condition has only one sub-state of interest, 
        /// where the condition is active or not. The values Severity, Description and AckRequired flag are the default values for those Conditions which uses this Definition. 
        /// The default values are used if no other values are specified with function ProcessConditionStateChages(). 
        /// </remarks>
        public static int AddSingleStateConditionDefinition(
                                    int categoryId,
                                    int conditionId,
                                    string name,
                                    string condition,
                                    int severity,
                                    string description,
                                    bool ackRequired)
        {
            if (addSingleStateConditionDefinition_ != null)
            {
                return addSingleStateConditionDefinition_(categoryId, conditionId, name, condition, severity, description, ackRequired);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds a Multi State Condition Definition to the Alarms&amp;Event Category.</summary>
        /// <param name="categoryId">Identifier of an existing Event Category.</param>
        /// <param name="conditionId">Identifier of the new Event Condition Definition. This ID must be unique within the Alarms&amp;Events Server.</param>
        /// <param name="name">Text string with the name of the Event Condition Definition. This name must be unique within the larms&amp;Events Server. </param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the Single State Condition Definition was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// Adds a definition to the Event Space which can be used by Multi State Conditions. A Multi State Condition has at least two sub-states of interest. 
        /// Sub-states are mutually exclusive, only one sub-state can be active. Use the function AddSubCondition() to add at least two Sub Condition 
        /// Definitions for the sub-states.  
        /// </remarks>
        public static int AddMultiStateConditionDefinition(
                                    int categoryId,
                                    int conditionId,
                                    string name)
        {
            if (addMultiStateConditionDefinition_ != null)
            {
                return addMultiStateConditionDefinition_(categoryId, conditionId, name);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds a Sub Condition Definition to an existing Multi State Condition Definition.</summary>
        /// <param name="conditionId">Identifier of an existing Event Condition Definition.</param>
        /// <param name="subConditionId">Identifier of the new Sub Condition Definition. This ID must be unique within the Event Condition Definition. 
        /// Value 0 is invalid because internally used.</param>
        /// <param name="name">Text string with the name of the Event Sub Condition Definition. This name must be unique within the larms&amp;Events Server. </param>
        /// <param name="condition">Text string with the condition represented by this definition.</param>
        /// <param name="severity">The urgency in the range of 1 ... 1000. This is a default value. </param>
        /// <param name="description">Text string with description. This text is used as message for the generated Events. This is a default value.</param>
        /// <param name="ackRequired">True if the Event Conditions which uses this Definition requires acknowledgement; otherwise False. This is a default value. </param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the Sub Condition Definition was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// This function must be called at least twice for each Multi State Condition Definition added with AddMultiStateConditionDef(). 
        /// A Multi State Condition has at least two sub-states of interest. Sub-states are mutually exclusive, only one sub-state can be active. The values Severity, 
        /// Description and AckRequired flag are the default values for those Conditions which uses this Definition. The default values are used if no other values are 
        /// specified with function ProcessConditionStateChages().  
        /// </remarks>
        public static int AddSubConditionDefinition(
                                    int conditionId,
                                    int subConditionId,
                                    string name,
                                    string condition,
                                    int severity,
                                    string description,
                                    bool ackRequired)
        {
            if (addSubConditionDefinition_ != null)
            {
                return addSubConditionDefinition_(conditionId, subConditionId, name, condition, severity, description, ackRequired);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds an Area to the Alarms&amp;Event Process Area Space.</summary>
        /// <param name="parentAreaId">Identifier of an existing Event Area. Use AREAID_ROOT to add an Area to the Root Area.</param>
        /// <param name="areaId">Identifier of the new Event Area. This ID must be unique within the Alarms&amp;Events Server.</param>
        /// <param name="name">Name of the Event Area. Do not use default delimiter characters. See comment below.</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the Single State Condition Definition was successfully added to the event space.
        /// </returns>
        /// <remarks>
        /// The name parts of a fully qualified Area Name are separated by '.' characters. This default value can be changed by the static function 
        /// SetDelimiter(). Be sure that the specified Event Names doesn't include any default delimiter characters.   
        /// </remarks>
        public static int AddArea(
                                    int parentAreaId,
                                    int areaId,
                                    string name)
        {
            if (addArea_ != null)
            {
                return addArea_(parentAreaId, areaId, name);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds an Event Source object to the Alarms&amp;Event Process Area Space.</summary>
        /// <param name="areaId">Identifier of an existing Event Area. Use AREAID_ROOT to add a top-level Event Source object.</param>
        /// <param name="sourceId">Identifier of the new Event Source. This ID must be unique within the Alarms&amp;Events Server.</param>
        /// <param name="sourceName">Name of the Event Source. If parameter 'multiSource' is true then this parameter specifies the fully qualified source name; otherwise only the partial source name.</param>
        /// <param name="multiSource">true if this Event Source object is shared by multiple Process Areas (see function AddExistingSource()); otherwise false.</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event source was successfully added to the process area space.
        /// </returns>
        /// <remarks>
        /// Each event generated by an Alarms&amp;Event Server has an associated source object. 
        /// </remarks>
        public static int AddSource(
                                    int areaId,
                                    int sourceId,
                                    string sourceName,
                                    Boolean multiSource)
        {
            if (addSource_!= null)
            {
                return addSource_(areaId, sourceId, sourceName, multiSource);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds an existing Event Source object to an additional Alarms&amp;Event Process Area Space.</summary>
        /// <param name="areaId">Identifier of an existing Event Area. Use AREAID_ROOT to add a top-level Event Source object.</param>
        /// <param name="sourceId">Identifier of an existing Event Source object. The object with this ID must be previously added to the Process Area Space by the 
        /// function AddSource() with an active 'multiSource' flag. </param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event source was successfully added to the process area space.
        /// </returns>
        /// <remarks>
        /// Use this function if an Event Source object should be a member of multiple Process Areas. 
        /// </remarks>
        public static int AddExistingSource(
                                    int areaId,
                                    int sourceId)
        {
            if (addExistingSource_ != null)
            {
                return addExistingSource_(areaId, sourceId);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Adds an Event Condition to the Alarms&amp;Event Process Area Space.</summary>
        /// <param name="sourceId">Identifier of an existing Event Source.</param>
        /// <param name="conditionDefinitionId">Identifier of an existing Event Condition Definition.</param>
        /// <param name="conditionId">Identifier of the new Event Condition. This ID must be unique within the Alarms&amp;Events Server.</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
        /// </returns>
        /// <remarks>
        /// An Event Condition associates a Condition Definition with an Event Source. If the state of a Condition changes then the server creates events. 
        /// </remarks>
        public static int AddCondition(
                                    int sourceId,
                                    int conditionDefinitionId,
                                    int conditionId)
        {
            if (addCondition_ != null)
            {
                return addCondition_(sourceId, conditionDefinitionId, conditionId);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Generates a Simple Event. </summary>
        /// <param name="categoryId">Identifier of an existing Event Category. Specifies the Event Category to which this Event belongs.</param>
        /// <param name="sourceId">Identifier of an existing Event Source. Specifies the object which generates the event notification.</param>
        /// <param name="message">Message text string which describes the Event.</param>
        /// <param name="severity">The urgency of the Event in the range of 1 ... 1000.</param>
        /// <param name="attributeCount">Number of attribute values specified in pvAttrValues. This number must be identical with the number of 
        /// attributes added to the specified Event Category. This parameter is only used for cross-check.</param>
        /// <param name="attributeValues">Array of attribute values. The order and the types must be identical with the attributes of the specified Event Category.</param> 
        /// <param name="timeStamp">Specifies the occurrence time of the event. If this parameter is a NULL pointer then the current time is used.</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
        /// </returns>
        /// <remarks>
        /// Use this function to generate a Single Event. A Single Events is not associated with an Event Condition. The generic server part uses the subscription 
        /// specific filters and forwards the Event to the subscriptions of all connected clients if the filters are passed.  
        /// </remarks>
        public static int ProcessSimpleEvent(
                                    int categoryId,
                                    int sourceId,
                                    string message,
                                    int severity,
                                    int attributeCount,
                                    object[] attributeValues,
                                    DateTime timeStamp)
        {
            if (processSimpleEvent_ != null)
            {
                return processSimpleEvent_(categoryId, sourceId, message, severity, attributeCount, attributeValues, timeStamp);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Generates a Tracking Event.</summary>
        /// <param name="categoryId">Identifier of an existing Event Category. Specifies the Event Category to which this Event belongs.</param>
        /// <param name="sourceId">Identifier of an existing Event Source. Specifies the object which generates the event notification.</param>
        /// <param name="message">Message text string which describes the Event.</param>
        /// <param name="severity">The urgency of the Event in the range of 1 ... 1000.</param>
        /// <param name="actorId">Text string which identifies the OPC Client which initiated the action resulting the tracking-related Event.</param>
        /// <param name="attributeCount">Number of attribute values specified in pvAttrValues. This number must be identical with the number of 
        /// attributes added to the specified Event Category. This parameter is only used for cross-check.</param>
        /// <param name="attributeValues">Array of attribute values. The order and the types must be identical with the attributes of the specified Event Category.</param> 
        /// <param name="timeStamp">Specifies the occurrence time of the event. If this parameter is a NULL pointer then the current time is used.</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
        /// </returns>
        /// <remarks>
        /// Use this function to generate a Tracking Event. A Tracking Events is not associated with an Event Condition. The generic server part uses the 
        /// subscription specific filters and forwards the Event to the subscriptions of all connected clients if the filters are passed. 
        /// </remarks>
        public static int ProcessTrackingEvent(
                                    int categoryId,
                                    int sourceId,
                                    string message,
                                    int severity,
                                    string actorId,
                                    int attributeCount,
                                    object[] attributeValues,
                                    DateTime timeStamp)
        {
            if (processTrackingEvent_ != null)
            {
                return processTrackingEvent_(categoryId, sourceId, message, severity, actorId, attributeCount, attributeValues, timeStamp);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Changes the state of one or more Event Conditions.</summary>
        /// <param name="count">Number of Conditions to be changed.</param>
        /// <param name="conditionStateChanges">Array of class ConditionChangeStates with the new condition states.</param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event condition was successfully added to the process area space.
        /// </returns>
        /// <remarks>
        /// Changes the state of one or more Event Conditions. If the state of a condition changes then the generic server part creates Event instances .The generic server part uses the subscription specific 
        /// filters and forwards the generated Events to the subscriptions of all connected clients if the filters are passed. 
        /// </remarks>
        public static int ProcessConditionStateChanges(
                                    int count,
                                    AeConditionState[] conditionStateChanges)
        {
            if (processConditionStateChanges_ != null)
            {
                return processConditionStateChanges_(count, conditionStateChanges);
            }
            return StatusCodes.BadNotImplemented;
        }

        /// <summary>Internally acknowledgement of an Event Condition.</summary>
        /// <param name="conditionId">Identifier of an existing Event Condition.</param>
        /// <param name="comment">Text string with a comment. A NULL pointer means there is no comment. This parameter is optional. </param>
        /// <returns>
        ///     A <see cref="StatusCodes">StatusCodes</see> code with the result of the operation.
        ///     Returns StatusCodes.Good if the event condition was successfully acknowledged.
        /// </returns>
        /// <remarks>
        /// Internally acknowledgement of a Condition by the server if it's no longer required to be acknowledged by a client. 
        /// The generic function part calls the function OnAckNotification(). 
        /// </remarks>
        public static int AckCondition(
                                    int conditionId,
                                    string comment)
        {
            if (ackCondition_ != null)
            {
                return ackCondition_(conditionId, comment);
            }
            return StatusCodes.BadNotImplemented;
        }

        #endregion

        #region  .NET API Generic Server Default Methods
        //---------------------------------------------------------------------
        //  .NET API Methods 
        // (Called by the generic server)
        //---------------------------------------------------------------------

        /// <summary>
        /// 	<para>This method is called from the generic server at startup. It passes the
        ///  callback methods supported by the generic server. These callback methods can be
        ///  called anytime to exchange data with the generic server.</para>
        /// 	<para>The DefineCallbacks method need not be overloaded or changed. The default
        ///  implementation stores the delegates for later callbacks.</para>
        /// </summary>
        /// <param name="addSimpleEventCategory">Adds a Simple Event Category to the Alarms&amp;Event Space.</param>
        /// <param name="addTrackingEventCategory">Adds a Tracking Event Category to the Alarms&amp;Event Space.</param>
        /// <param name="AddConditionEventCategory">Adds a Condition Event Category to the Alarms&amp;Event Space.</param>
        /// <param name="addEventAttribute">Adds a vendor specific Attribute to the Alarms&amp;Event Category.</param>
        /// <param name="addSingleStateConditionDefinition">Adds a Single State Condition Definition to the Alarms&amp;Event Category.</param>
        /// <param name="addMultiStateConditionDefinition">Adds a Multi State Condition Definition to the Alarms&amp;Event Category.</param>
        /// <param name="addSubConditionDefinition">Adds a Sub Condition Definition to an existing Multi State Condition Definition.</param>
        /// <param name="addArea">Adds an Area to the Alarms&amp;Event Process Area Space.</param>
        /// <param name="addSource">Adds an Event Source object to the Alarms&amp;Event Process Area Space.</param>
        /// <param name="addExistingSource">Adds an existing Event Source object to an additional Alarms&amp;Event Process Area Space.</param>
        /// <param name="addCondition">Adds an Event Condition to the Alarms&amp;Event Process Area Space.</param>
        /// <param name="processSimpleEvent">Generates a Simple Event.</param>
        /// <param name="processTrackingEvent">Generates a Tracking Event.</param>
        /// <param name="processConditionStateChanges">Changes the state of one or more Event Conditions.</param>
        /// <param name="ackCondition">Internally acknowledgement of an Event Condition.</param>
        public virtual void OnDefineAeCallbacks(AddSimpleEventCategory addSimpleEventCategory, AddTrackingEventCategory addTrackingEventCategory, AddConditionEventCategory AddConditionEventCategory, AddEventAttribute addEventAttribute, AddSingleStateConditionDefinition addSingleStateConditionDefinition, AddMultiStateConditionDefinition addMultiStateConditionDefinition, AddSubConditionDefinition addSubConditionDefinition, AddArea addArea, AddSource addSource, AddExistingSource addExistingSource, AddCondition addCondition, ProcessSimpleEvent processSimpleEvent, ProcessTrackingEvent processTrackingEvent, ProcessConditionStateChanges processConditionStateChanges, AckCondition ackCondition)
        {
            addSimpleEventCategory_ = addSimpleEventCategory;
            addTrackingEventCategory_ = addTrackingEventCategory;
            addConditionEventCategory_ = AddConditionEventCategory;
            addEventAttribute_ = addEventAttribute;
            addSingleStateConditionDefinition_ = addSingleStateConditionDefinition;
            addMultiStateConditionDefinition_ = addMultiStateConditionDefinition;
            addSubConditionDefinition_ = addSubConditionDefinition;
            addArea_ = addArea;
            addSource_ = addSource;
            addExistingSource_ = addExistingSource;
            addCondition_ = addCondition;
            processSimpleEvent_ = processSimpleEvent;
            processTrackingEvent_ = processTrackingEvent;
            processConditionStateChanges_ = processConditionStateChanges;
            ackCondition_ = ackCondition;
        }

        /// <summary>
        /// 	<para>This method is called from the generic server at startup for normal operation or for registration. It provides server registry information for this
        /// application required for DCOM registration. The generic server registers the OPC server accordingly.</para>
        /// 	<para>The default implementation in <em>ClassicBaseNodeManager</em> returns an empty configuration. The method can be replaced by overriding it in the
        /// <em>ClassicNodeManager</em>.</para>
        /// </summary>
        /// <remarks>
        /// 	<para>The default implementation in ClassicBaseNodeManager.cs returns an empty configuration. The CLSID definitions need to be unique and can be created with
        /// the Visual Studio <em>Create GUID</em> tool.</para>
        /// </remarks>
        /// <returns>Definition structure</returns>
        /// <example>
        /// 	<para style="MARGIN-RIGHT: 0px" dir="ltr">
        /// 		<font color="blue" size="2" face="Consolas">
        /// 			<font color="blue" size="2" face="Consolas">
        /// 				<font color="blue" size="2" face="Consolas">public</font>
        /// 			</font>
        /// 		</font>
        /// 		<font color="blue" size="2" face="Consolas">
        /// 			<font color="blue" size="2" face="Consolas">
        /// 				<font color="blue" size="2" face="Consolas">override</font>
        /// 			</font>
        /// 		</font>
        /// 		<font color="#2B91AF" size="2" face="Consolas">
        /// 			<font color="#2B91AF" size="2" face="Consolas">
        /// 				<font color="#2B91AF" size="2" face="Consolas">ClassicServerRegistryInfo</font>
        /// 			</font>
        /// 		</font>
        /// 		<font size="2" face="Consolas">
        /// 			<font size="2" face="Consolas">OnGetAeServerDefinition()<br/>
        /// {</font>
        /// 		</font>
        /// 	</para>
        /// 	<blockquote style="MARGIN-RIGHT: 0px" dir="ltr">
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#region</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">General
        ///     Settings</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer.ClsIdApp
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"{9236F2A6-96EA-4D44-8C42-3A6DDA061BC6}"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer.CompanyName
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware GmbH"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#endregion</font>
        /// 						</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#region</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">DA Server
        ///     registry definitions</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer.ClsIdServer
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"{DD2E86BD-266A-43F9-BDFE-3A0B40B94C20}"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer.PrgIdServer
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware.AeSample"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer.PrgIdCurrServer
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware.AeSample.20"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer.ServerName
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware OPC AE Sample Server"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer.CurrServerName
        ///     =</font>
        /// 					</font>
        /// 					<font color="#A31515" size="2" face="Consolas">
        /// 						<font color="#A31515" size="2" face="Consolas">
        /// 							<font color="#A31515" size="2" face="Consolas">"Technosoftware OPC AE Sample Server V2.0"</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">#endregion</font>
        /// 						</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 		<para>
        /// 			<font size="2" face="Consolas">
        /// 				<font size="2" face="Consolas">
        /// 					<font color="blue" size="2" face="Consolas">
        /// 						<font color="blue" size="2" face="Consolas">
        /// 							<font color="blue" size="2" face="Consolas">return</font>
        /// 						</font>
        /// 					</font>
        /// 					<font size="2" face="Consolas">
        /// 						<font size="2" face="Consolas">AeServer;</font>
        /// 					</font>
        /// 				</font>
        /// 			</font>
        /// 		</para>
        /// 	</blockquote>
        /// 	<para dir="ltr">
        /// 		<font size="2" face="Consolas">
        /// 			<font size="2" face="Consolas">}</font>
        /// 		</font>
        /// 	</para>
        /// </example>
        public virtual ClassicServerDefinition OnGetAeServerDefinition()
        {
            AeServer = new ClassicServerDefinition();

            return AeServer;
        }

        /// <summary>
        /// Notification if an Event Condition has been acknowledged.  
        /// </summary>
        /// <param name="conditionId">Event Category Identifier.</param>
        /// <param name="subConditionId">Sub Condition Definition Identifier. It's 0 for Single State Conditions.</param>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        /// <remarks>Called by the generic server part if the Event Condition specified by the parameters has been acknowledged. 
        /// This function is called if the Event Condition is successfully acknowledged but before the indication events are sent 
        /// to the clients. If this function fails then the error code will be returned to the client and no indication events will be generated.</remarks>
        public virtual int OnAckNotification(int conditionId, int subConditionId)
        {
            return StatusCodes.Good;
        }

        /// <summary>
        /// Returns information about the OPC DA Item corresponding to an Event Attribute.  
        /// </summary>
        /// <param name="conditionId">Event Category Identifier.</param>
        /// <param name="subConditionId">Sub Condition Definition Identifier. It's 0 for Single State Conditions.</param>
        /// <param name="attributeId">Event Attribute Identifier.</param>
        /// <param name="itemId">Pointer to where the text string with the ItemID of the associated OPC DA Item will be saved. 
        /// Use a null string if there is no OPC Item corresponding to the Event Attribute specified by the parameters conditionId, subConditionId and attributeId. </param>
        /// <param name="nodeName">Pointer to where the text string with the network node name of the associated OPC Data Access Server will be saved. 
        /// Use a null string if the server is running on the local node. </param>
        /// <param name="clsid">CLSID of the associated Data Access Server will be saved. Use the value null if there is no associated OPC Item.</param>
        /// <returns>A <see cref="StatusCodes"/> code with the result of the operation.</returns>
        /// <remarks>Called by the generic server part to get information about the OPC DA Item corresponding to an Event Attribute.</remarks>
        public virtual int OnTranslateToItemId(int conditionId, int subConditionId, int attributeId, out string itemId, out string nodeName, out string clsid)
        {
            itemId = null;
            nodeName = null;
            clsid = null;
            return StatusCodes.Good;
        }
        #endregion
        
        #endregion

    } 

}
