/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System.Collections.Generic;


namespace IEC61850
{
    namespace SCL
    {

        namespace DataModel
        {

            public class IEDModelNode
            {
                protected string name;

                protected string objRef;

                private IEDModelNode parent;

                public IEDModelNode Parent
                {
                    get
                    {
                        return parent;
                    }
                }

                public string ObjRef
                {
                    get
                    {
                        return objRef;
                    }
                }

                public string Name
                {
                    get
                    {
                        return name;
                    }
                    set
                    {
                        name = value;
                        //change objRef;
                        int index = objRef.LastIndexOf(".");
                        if (index >= 0)
                            objRef = objRef.Substring(0, index + 1) + value;


                    }
                }

                public string IedName
                {
                    get
                    {
                        if (this is IEDDataModel)
                            return Name;
                        else
                            return parent.IedName;
                    }
                }

                public IEDModelNode(string name, IEDModelNode parent)
                {
                    this.name = name;
                    this.parent = parent;
                }
            }

            public class IEDDataModel : IEDModelNode
            {
                private List<LogicalDevice> logicalDevices = new List<LogicalDevice>();

                public List<LogicalDevice> LogicalDevices
                {
                    get
                    {
                        return logicalDevices;
                    }
                }

                public IEDDataModel(string name) : base(name, null)
                {
                }

                private IEDModelNode GetChildNode(IEDModelNode node, string objRefPart)
                {
                    string[] tokens = objRefPart.Split(new char[] { '.' }, 2);

                    if (tokens.Length > 0)
                    {
                        if (node is LogicalDevice)
                        {
                            LogicalDevice ld = node as LogicalDevice;

                            foreach (LogicalNode ln in ld.LogicalNodes)
                            {
                                if (ln.Name.Equals(tokens[0]))
                                {
                                    if (tokens.Length == 1)
                                        return ln;
                                    else
                                    {
                                        return GetChildNode(ln, tokens[1]);
                                    }
                                }
                            }
                        }
                        else if (node is LogicalNode)
                        {
                            LogicalNode ln = node as LogicalNode;

                            foreach (DataObject dobj in ln.DataObjects)
                            {
                                if (dobj.Name.Equals(tokens[0]))
                                {
                                    if (tokens.Length == 1)
                                        return dobj;
                                    else
                                    {
                                        return GetChildNode(dobj, tokens[1]);
                                    }
                                }
                            }

                        }
                        else if (node is DataObject)
                        {
                            DataObject dobj = node as DataObject;

                            foreach (DataObjectOrAttribute doa in dobj.DataObjectsAndAttributes)
                            {
                                if (doa.Name.Equals(tokens[0]))
                                {
                                    if (tokens.Length == 1)
                                        return doa;
                                    else
                                    {
                                        return GetChildNode(doa, tokens[1]);
                                    }
                                }
                            }
                        }
                        else if (node is DataAttribute)
                        {
                            DataAttribute da = node as DataAttribute;

                            foreach (DataAttribute sda in da.SubDataAttributes)
                            {
                                if (sda.Name.Equals(tokens[0]))
                                {
                                    if (tokens.Length == 1)
                                        return sda;
                                    else
                                    {
                                        return GetChildNode(sda, tokens[1]);
                                    }
                                }
                            }
                        }

                        return null;
                    }
                    else
                        return null;
                }

                /// <summary>
                /// Get the model node with the provided object reference
                /// </summary>
                /// <param name="objRef">object reference in the format LD/LN.DO[.SDO].DA[.SDA]</param>
                /// <returns></returns>
                public IEDModelNode GetModelNode(string objRef)
                {
                    string[] tokens = objRef.Split(new char[] { '/' }, 2);

                    if (tokens.Length > 0)
                    {
                        foreach (LogicalDevice ld in logicalDevices)
                        {
                            if (ld.ObjRef.Equals(tokens[0]))
                            {
                                if (tokens.Length == 1)
                                    return ld;
                                else
                                {
                                    return GetChildNode(ld, tokens[1]);
                                }
                            }
                        }

                        return null;
                    }
                    else
                        return null;

                }
            }

            public class LogicalDevice : IEDModelNode
            {
                private SclDocument SclDocument = null;
                private List<LogicalNode> logicalNodes = new List<LogicalNode>();

                private SclLDevice sclLDevice;
                private Namespace nameSpace = null;

                private string inst;


                public void UpdateName()
                {
                    string name = "";

                    if (IedName != null)
                        name += IedName;

                    if (inst != null)
                        name += inst;

                    this.name = name;
                    objRef = name;
                }

                public SclLDevice SclLDevice
                {
                    get => sclLDevice; set => sclLDevice = value;

                }

                public string Inst
                {
                    get
                    {
                        return inst;
                    }
                    set
                    {
                        inst = value;
                        UpdateName();
                    }
                }

                public List<LogicalNode> LogicalNodes
                {
                    get
                    {
                        return logicalNodes;
                    }
                }

                public Namespace NameSpace { get => nameSpace; set => nameSpace = value; }

                public LogicalNode AddLogicalNode(SclLN ln)
                {
                    LogicalNode logicalNode = new LogicalNode(ln, this);

                    SclDocument.AddDataObjectsToLogicalNode(logicalNode, ln);

                    LogicalNodes.Add(logicalNode);

                    return logicalNode;
                }

                public LogicalDevice(SclDocument sclDocument, SclLDevice sclLDevice, string name, string inst, IEDModelNode parent) : base(name, parent)
                {
                    SclDocument = sclDocument;
                    this.name = name;
                    this.inst = inst;
                    objRef = this.name;
                    this.sclLDevice = sclLDevice;
                }
            }

            public class LogicalNode : IEDModelNode
            {
                private string prefix;
                private string lnClass;
                private string inst;
                private Namespace nameSpace = null;

                private SclLN sclElement = null;

                private List<DataSet> dataSets = null;
                private List<Log> logs = null;
                private List<ReportControl> reportControls = null;
                private List<GSEControl> gseControls = null;
                private List<SMVControl> sMVControls = null;
                private List<LogControl> logControls = null;
                private List<DataObject> dataObjects = new List<DataObject>();
                private Inputs inputs = null;
                private SclSettingControl settingControl = null;

                public SclLN SclElement { get => sclElement; set => sclElement = value; }

                public void UpdateName()
                {
                    string name = "";

                    if (prefix != null)
                        name += prefix;

                    name += lnClass;

                    if (inst != null)
                        name += inst;

                    this.name = name;
                }

                public string Prefix
                {
                    get
                    {
                        return prefix;
                    }
                    set
                    {
                        prefix = value;
                    }
                }

                public string LnClass
                {
                    get
                    {
                        return lnClass;
                    }
                    set
                    {
                        lnClass = value;
                    }
                }

                public string Inst
                {
                    get
                    {
                        return inst;
                    }
                    set
                    {
                        inst = value;
                    }
                }

                public List<DataSet> DataSets
                {
                    get
                    {
                        if (dataSets == null)
                        {
                            dataSets = new List<DataSet>();

                            if (sclElement != null)
                            {
                                foreach (SclDataSet sclDataSet in sclElement.DataSets)
                                {
                                    dataSets.Add(new DataSet(sclDataSet, this));
                                }
                            }
                        }

                        return dataSets;
                    }
                }

                public List<Log> Logs
                {
                    get
                    {
                        if (logs == null)
                        {
                            logs = new List<Log>();

                            if (sclElement != null)
                            {
                                foreach (SclLog sclLog in sclElement.Logs)
                                {
                                    logs.Add(new Log(sclLog, this));
                                }
                            }
                        }

                        return logs;
                    }
                }

                public List<ReportControl> ReportControlBlocks
                {
                    get
                    {
                        if (reportControls == null)
                        {
                            reportControls = new List<ReportControl>();

                            if (sclElement != null)
                            {
                                foreach (SclReportControl sclReportControl in sclElement.ReportControls)
                                {
                                    reportControls.Add(new ReportControl(sclReportControl, this));
                                }
                            }
                        }

                        return reportControls;
                    }
                }

                public List<GSEControl> GSEControls
                {
                    get
                    {
                        if (gseControls == null)
                        {
                            gseControls = new List<GSEControl>();

                            if (sclElement != null)
                            {
                                foreach (SclGSEControl sclGSEControl in sclElement.GSEControls)
                                {
                                    gseControls.Add(new GSEControl(sclGSEControl, this));
                                }
                            }
                        }

                        return gseControls;
                    }
                }

                public List<SMVControl> SMVControls
                {
                    get
                    {
                        if (sMVControls == null)
                        {
                            sMVControls = new List<SMVControl>();

                            if (sclElement != null)
                            {
                                foreach (SclSMVControl sclSMVControl in sclElement.SclSMVControls)
                                {
                                    sMVControls.Add(new SMVControl(sclSMVControl, this));
                                }
                            }
                        }

                        return sMVControls;
                    }
                }

                public List<LogControl> LogControls
                {
                    get
                    {
                        if (logControls == null)
                        {
                            logControls = new List<LogControl>();

                            if (sclElement != null)
                            {
                                foreach (SclLogControl sclLogControl in sclElement.LogControls)
                                {
                                    logControls.Add(new LogControl(sclLogControl, this));
                                }
                            }
                        }

                        return logControls;
                    }
                }

                public Inputs Inputs
                {
                    get
                    {
                        if (inputs == null)
                        {
                            if (sclElement != null)
                            {
                                if (sclElement.Inputs != null)
                                {
                                    inputs = sclElement.Inputs;
                                    inputs.Parent = this;

                                    foreach (SclExtRef extRef in inputs.ExtRefs)
                                        extRef.Parent = inputs;
                                }
                            }
                        }

                        return inputs;
                    }
                    set
                    {
                        inputs = value;
                        if (inputs != null)
                            inputs.Parent = this;
                    }
                }

                public SclSettingControl SettingControl
                {
                    get
                    {
                        if (settingControl == null)
                        {
                            if (sclElement != null)
                            {
                                if (sclElement.SettingControl != null)
                                {
                                    settingControl = sclElement.SettingControl;
                                    settingControl.Parent = this;
                                }
                            }
                        }

                        return settingControl;
                    }
                    set
                    {
                        settingControl = value;
                        if (settingControl != null)
                            settingControl.Parent = this;
                    }
                }

                public List<DataObject> DataObjects
                {
                    get
                    {
                        return dataObjects;
                    }

                }

                public Namespace NameSpace { get => nameSpace; set => nameSpace = value; }

                public LogicalNode(string prefix, string lnClass, string inst, IEDModelNode parent) : base(null, parent)
                {
                    this.prefix = prefix;
                    this.lnClass = lnClass;
                    this.inst = inst;
                    UpdateName();
                    objRef = parent.Name + "/" + name;
                }

                public LogicalNode(SclLN ln, IEDModelNode parent) : base(null, parent)
                {
                    prefix = ln.Prefix;
                    lnClass = ln.LnClass;
                    inst = ln.Inst;
                    //this.name = ln.InstanceName;
                    //if (ln.InstanceName == null)
                    UpdateName();
                    objRef = parent.Name + "/" + name;
                    sclElement = ln;
                }
            }

            public class DataObjectOrAttribute : IEDModelNode
            {
                public DataObjectOrAttribute(string name, IEDModelNode parent) : base(name, parent)
                {
                }
            }

            public class DataObject : DataObjectOrAttribute
            {
                private List<DataObjectOrAttribute> dataObjectsAndAttributes = new List<DataObjectOrAttribute>();

                private int count = 0;
                private SclDOType doType;
                private bool trans = false;

                public bool IsTransiente
                {
                    get
                    {
                        return trans;
                    }
                }

                public SclDOType DOType
                {
                    get
                    {
                        return doType;
                    }

                }

                public int Count
                {
                    get
                    {
                        return count;
                    }
                }

                public List<DataObjectOrAttribute> DataObjectsAndAttributes
                {
                    get
                    {
                        return dataObjectsAndAttributes;
                    }
                }

                public DataObject(SclDataObjectDefinition dod, SclDOType doType, IEDModelNode parent) : base(dod.Name, parent)
                {
                    count = dod.Count;
                    this.doType = doType;
                    objRef = parent.ObjRef + "." + name;

                    trans = dod.IsTransient;
                }

                private bool HasChildWithFc(DataObject dobj, SclFC fc)
                {
                    foreach (DataObjectOrAttribute doa in dobj.DataObjectsAndAttributes)
                    {
                        if (doa is DataAttribute)
                        {
                            DataAttribute da = doa as DataAttribute;

                            if (da.Fc == fc)
                                return true;
                        }
                        else if (doa is DataObject)
                        {
                            DataObject sdo = doa as DataObject;

                            if (HasChildWithFc(sdo, fc))
                                return true;
                        }
                    }

                    return false;
                }

                public bool HasChildWithFc(SclFC fc)
                {
                    return HasChildWithFc(this, fc);
                }


            }

            public class DataAttribute : DataObjectOrAttribute
            {
                public List<DataAttribute> subDataAttributes = new List<DataAttribute>();

                private AttributeType attributeType;
                private SclFC fc;
                private int count = 0;
                private SclDataAttributeDefinition definition = null;

                public List<DataAttribute> SubDataAttributes
                {
                    get
                    {
                        return subDataAttributes;
                    }
                }

                public SclDataAttributeDefinition Definition
                {
                    get
                    {
                        return definition;
                    }
                }

                public AttributeType AttributeType
                {
                    get
                    {
                        return attributeType;
                    }
                }

                public SclFC Fc
                {
                    get
                    {
                        return fc;
                    }
                }

                public int Count
                {
                    get
                    {
                        return count;
                    }
                }

                public DataAttribute(string name, IEDModelNode parent, SclFC fc, AttributeType bType, int count) : base(name, parent)
                {
                    this.fc = fc;
                    attributeType = bType;
                    this.count = count;

                    if (attributeType == AttributeType.CONSTRUCTED)
                        subDataAttributes = new List<DataAttribute>();

                    objRef = parent.ObjRef + "." + name;
                }

                public DataAttribute(string name, IEDModelNode parent, SclFC fc, AttributeType bType, int count, SclDataAttributeDefinition def) : base(name, parent)
                {
                    this.fc = fc;
                    attributeType = bType;
                    this.count = count;
                    definition = def;

                    if (attributeType == AttributeType.CONSTRUCTED)
                        subDataAttributes = new List<DataAttribute>();

                    if (objRef == null)
                        objRef = parent.ObjRef + "." + name;
                }
            }

            public class DataSet : IEDModelNode
            {
                private SclDataSet sclDataSet;

                public DataSet(string name, IEDModelNode parent) : base(name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                }

                public DataSet(SclDataSet sclDataSet, IEDModelNode parent) : base(sclDataSet.Name, parent)
                {
                    this.sclDataSet = sclDataSet;
                    objRef = parent.ObjRef + "." + name;
                }

                public SclDataSet SclDataSet { get => sclDataSet; set => sclDataSet = value; }
            }

            public class Log : IEDModelNode
            {
                private SclLog sclLog;

                public Log(string name, IEDModelNode parent) : base(name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                }

                public Log(SclLog sclLog, IEDModelNode parent) : base(sclLog.Name, parent)
                {
                    this.sclLog = sclLog;
                    objRef = parent.ObjRef + "." + name;
                }

                public SclLog SclLog { get => sclLog; set => sclLog = value; }
            }


            public class ReportControl : IEDModelNode
            {
                private SclReportControl sclReportControl;
                private bool buffered = false;

                public SclReportControl SclReportControl { get => sclReportControl; set => sclReportControl = value; }
                public bool Buffered { get => buffered; set => buffered = value; }

                public ReportControl(string name, bool buffered, IEDModelNode parent) : base(name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                    this.buffered = buffered;
                }


                public ReportControl(SclReportControl sclReportControl, IEDModelNode parent) : base(sclReportControl.Name, parent)
                {
                    this.sclReportControl = sclReportControl;

                    buffered = sclReportControl.Buffered;

                    objRef = parent.ObjRef + "." + name;
                }
            }

            public class GSEControl : IEDModelNode
            {
                private SclGSEControl sclGSEControl;

                public GSEControl(string name, IEDModelNode parent) : base(name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                }

                public GSEControl(SclGSEControl sclGSEControl, IEDModelNode parent) : base(sclGSEControl.Name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                    this.sclGSEControl = sclGSEControl;
                }

                public SclGSEControl SclGSEControl { get => sclGSEControl; set => sclGSEControl = value; }
            }

            public class SMVControl : IEDModelNode
            {
                private SclSMVControl sclSMVControl;

                public SMVControl(string name, IEDModelNode parent) : base(name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                }

                public SMVControl(SclSMVControl sclSMVControl, IEDModelNode parent) : base(sclSMVControl.Name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                    this.sclSMVControl = sclSMVControl;
                }

                public SclSMVControl SclSMVControl { get => sclSMVControl; set => sclSMVControl = value; }
            }

            public class LogControl : IEDModelNode
            {
                private SclLogControl sclLogControl;

                public LogControl(string name, IEDModelNode parent) : base(name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                }

                public LogControl(SclLogControl sclLogControl, IEDModelNode parent) : base(sclLogControl.Name, parent)
                {
                    objRef = parent.ObjRef + "." + name;
                    this.sclLogControl = sclLogControl;
                }

                public SclLogControl SclLogControl { get => sclLogControl; set => sclLogControl = value; }
            }
        }
    }
}
