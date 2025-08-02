/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 *  
 *  All rights reserved.
 */

using IEC61850.SCL.DataModel;
using System;
using System.Collections.Generic;
using System.Text.RegularExpressions;
using System.Xml;

namespace IEC61850.SCL
{

    public class NodeExtraAttributes
    {
        private string name;
        private string value;

        public NodeExtraAttributes(string name, string value)
        {
            Name = name;
            Value = value;

        }

        public string Name { get => name; set => name = value; }
        public string Value { get => value; set => this.value = value; }
    }
    public class SclIED
    {
        private List<SclAccessPoint> accessPoints = new List<SclAccessPoint>();
        private SclServices sclServices;
        private XmlDocument xmlDocument;
        private SclDocument sclDocument;
        private XmlNamespaceManager nsManager;
        public List<NodeExtraAttributes> NodeExtraAttributes = new List<NodeExtraAttributes>();


        internal XmlNode xmlNode;

        public SclServices SclServices
        {
            get
            {
                return sclServices;
            }

            set
            {
                sclServices = value;
            }
        }

        public string OriginalSclRevision
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "originalSclRevision");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "originalSclRevision", value);
            }
        }

        public string OriginalSclVersion
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "originalSclVersion");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "originalSclVersion", value);
            }
        }

        public string OriginalSclRelease
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "originalSclRelease");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "originalSclRelease", value);
            }
        }

        public string EngRight
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "engRight");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "engRight", value);
            }
        }

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "name", value);
            }
        }

        public string Type
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "type");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "type", value);
            }
        }

        public string Manufacturer
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "manufacturer");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "manufacturer", value);
            }
        }

        public string ConfigVersion
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "configVersion");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "configVersion", value);
            }
        }

        public string Owner
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "owner");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "owner", value);
            }
        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "desc", value);
            }
        }

        public List<SclAccessPoint> AccessPoints
        {
            get
            {

                if (accessPoints == null)
                    accessPoints = new List<SclAccessPoint>();

                return accessPoints;
            }
        }

        public static bool CheckIEDName(string iedName)
        {

            bool isValid = true;

            // 1.) Starting with alpha char

            // 2.) only alphanumeric chars and underscore

            // 3.) maximum 64 characters
            if (iedName.Length > 64 || !Regex.IsMatch(iedName[0].ToString(), @"^[a-zA-Z]"))
                isValid = false;

            return isValid;
        }

        public bool DeleteAccessPoint(string name)
        {
            if (name != null)
            {
                SclAccessPoint sclAccessPoint = accessPoints.Find(x => x.Name == name);

                if (sclAccessPoint != null)
                {
                    accessPoints.Remove(sclAccessPoint);

                    xmlNode.RemoveChild(sclAccessPoint.XmlNode);

                    return true;

                }

            }

            return false;

        }

        public bool DeleteServices()
        {
            if (sclServices != null)
            {
                xmlNode.RemoveChild(sclServices.XmlNode);
                sclServices = null;

                return true;

            }

            return false;

        }

        public SclIED(SclDocument SclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclDocument.XmlDocument;
            sclDocument = SclDocument;
            this.nsManager = nsManager;

            if (xmlNode.Attributes != null)
            {
                foreach (XmlAttribute xmlAttribute in xmlNode.Attributes)
                {
                    if (xmlAttribute.Name != "name" && xmlAttribute.Name != "desc" &&
                        xmlAttribute.Name != "type" &&
                        xmlAttribute.Name != "manufacturer" && xmlAttribute.Name != "configVersion" &&
                        xmlAttribute.Name != "originalSclVersion"
                        && xmlAttribute.Name != "originalSclRevision"
                        && xmlAttribute.Name != "originalSclRelease"
                        && xmlAttribute.Name != "engRight"
                         && xmlAttribute.Name != "owner")
                    {
                        NodeExtraAttributes.Add(new SCL.NodeExtraAttributes(xmlAttribute.Name, xmlAttribute.Value));
                    }
                }
            }

            if (Name == null)
            {
                SclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No name attribute!", this, "MissingName");
            }
            else
            {
                if (CheckIEDName(Name) == false)
                    SclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "Invalid IED name \"" + Name + "\"", this, "InvalidName");
            }

            XmlNodeList apNodes = xmlNode.SelectNodes("scl:AccessPoint", nsManager);

            if (apNodes.Count < 1)
            {
                //accessPoints = null;
                SclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No AccessPoint in IED \"" + Name + "\"", this, "accessPoint");
            }

            foreach (XmlNode apNode in apNodes)
            {
                accessPoints.Add(new SclAccessPoint(SclDocument, apNode, nsManager));
            }

            XmlNode serviceNode = xmlNode.SelectSingleNode("scl:Services", nsManager);
            if (serviceNode == null)
            {
                sclServices = null;

                if (accessPoints != null)
                {
                    if (!accessPoints.Exists(x => x.SclServices != null))
                        SclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No Services in IED \"" + Name + "\"", this, "service");

                }
                //SclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No Service in IED \"" + Name + "\"", this, "service");
            }
            else
            {
                sclServices = new SclServices(SclDocument, serviceNode, nsManager);
            }


            if (AccessPoints != null)
            {
                foreach (SclAccessPoint ap in AccessPoints)
                {
                    if (ap.ServerAt != null)
                    {
                        bool serverAtExists = false;
                        foreach (SclAccessPoint apCheck in AccessPoints)
                        {
                            if (apCheck.Name == ap.ServerAt.ApName)
                            {
                                serverAtExists = true;
                            }
                        }

                        if (serverAtExists == false)
                        {
                            XmlNode serverAtNode = ap.ServerAt.XmlNode;
                            SclDocument.AddIssue(serverAtNode, "ERROR", "Model integrity", "Referenced Access Point " + ap.ServerAt.ApName + " does not exist", this, "accessPoint");
                        }
                    }
                }

            }

        }

    }

    public class SclAccessPoint
    {
        private SclServer server = null;
        private SclServices sclServices = null;
        private SclServerAt serverAt = null;
        private XmlDocument xmlDocument;
        private List<SclLN> logicalNodes = new List<SclLN>();
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        private XmlNamespaceManager nsManager;

        public XmlNode XmlNode
        {
            get
            {
                return xmlNode;
            }
        }

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "name", value);
            }
        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "desc", value);
            }
        }

        public string Router
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "router");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "router", value);
            }
        }

        public string Clock
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "clock");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "clock", value);
            }
        }

        public List<SclLN> LogicalNodes
        {
            get
            {
                return logicalNodes;
            }
        }

        public SclServer Server
        {
            get
            {
                return server;
            }
        }

        public SclServerAt ServerAt
        {
            get
            {
                return serverAt;
            }
        }

        public SclServices SclServices
        {
            get
            {
                return sclServices;
            }
        }

        public bool DeleteServices()
        {
            if (sclServices != null)
            {
                xmlNode.RemoveChild(sclServices.XmlNode);
                sclServices = null;

                return true;

            }

            return false;

        }

        public SclAccessPoint(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            this.xmlNode = xmlNode;
            sclDocument = SclxmlDocument;
            this.nsManager = nsManager;
            bool serverInFile = false;
            bool serverAtInFile = false;
            bool LNodesInFile = false;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr == null)
            {
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No name attribute", this, "MissingName");
            }

            XmlNode serviceNode = xmlNode.SelectSingleNode("scl:Services", nsManager);
            if (serviceNode == null)
            {
                sclServices = null;
            }
            else
            {
                sclServices = new SclServices(sclDocument, serviceNode, nsManager);
            }


            XmlNode serverNode = xmlNode.SelectSingleNode("scl:Server", nsManager);

            if (serverNode != null)
            {
                server = new SclServer(SclxmlDocument, serverNode, nsManager);
                serverInFile = true;
            }
            else
            {
                server = null;
            }

            XmlNode serverAtNode = xmlNode.SelectSingleNode("scl:ServerAt", nsManager);

            if (serverAtNode != null)
            {
                serverAt = new SclServerAt(SclxmlDocument, serverAtNode, nsManager);
                serverAtInFile = true;
            }
            else
            {
                serverAt = null;
            }

            XmlNodeList lnNodes = xmlNode.SelectNodes("scl:LN", nsManager);

            if (lnNodes != null)
            {
                foreach (XmlNode lnNode in lnNodes)
                    logicalNodes.Add(new SclLN(SclxmlDocument, lnNode, nsManager));

                if (lnNodes.Count > 0)
                    LNodesInFile = true;
            }
            else
            {
                logicalNodes = null;
            }

            if ((serverInFile == true && serverAtInFile == true) || (serverInFile == true && LNodesInFile == true) || (serverAtInFile == true && LNodesInFile == true))
            {
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "Access point contains more than one server description", this, "server");
            }
        }

    }

    public class SclServer
    {
        private SclAuthentication authentication = null;
        private List<SclLDevice> logicalDevices = new List<SclLDevice>();
        private XmlDocument xmlDocument;
        private SclDocument sclDocument;
        private XmlNode xmlNode;
        private XmlNamespaceManager nsManager;

        public SclAuthentication Authentication
        {
            get
            {
                return authentication;
            }
            set
            {
                authentication = value;

            }
        }

        public List<SclLDevice> LogicalDevices
        {
            get
            {
                return logicalDevices;
            }
        }

        public XmlNode XmlNode
        {
            get
            {
                return xmlNode;
            }
        }

        public void DeleteLogicalDevice(SclLDevice sclLD)
        {
            xmlNode.RemoveChild(sclLD.XmlNode);

            logicalDevices.Remove(sclLD);
        }

        public SclServer(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            sclDocument = SclxmlDocument;
            this.xmlNode = xmlNode;
            this.nsManager = nsManager;

            XmlNode authNode = xmlNode.SelectSingleNode("scl:Authentication", nsManager);

            if (authNode == null)
            {
                authentication = null;
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "Server has no Authentication", this, "authentication");
            }

            if (authNode != null)
                authentication = new SclAuthentication(authNode, nsManager);

            XmlNodeList ldNodes = xmlNode.SelectNodes("scl:LDevice", nsManager);

            if (ldNodes.Count < 1)
            {
                //logicalDevices = null;
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "Server has no LDevice", this, "lDevice");
            }

            foreach (XmlNode ldNode in ldNodes)
                logicalDevices.Add(new SclLDevice(SclxmlDocument, ldNode, nsManager));
        }

    }

    public class SclServerAt
    {
        private XmlDocument xmlDocument;
        private string apName;
        public XmlNode XmlNode;
        private SclDocument sclDocument;
        private XmlNamespaceManager nsManager;

        public string ApName
        {
            get { return XmlHelper.GetAttributeValue(XmlNode, "apName"); }

            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, XmlNode, "apName", value); }
        }

        public SclServerAt(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            XmlNode = xmlNode;

            XmlAttribute apNameAttr = xmlNode.Attributes["apName"];

            if (apNameAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No apName attribute in ServerAt", this, "MissingApName");

        }

    }


    public class SclAuthentication
    {

        public SclAuthentication(XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
        }
    }

    public class SclLDevice
    {
        private string inst;
        private string ldName = null;
        private string desc = null;
        private List<SclLN> logicalNodes = new List<SclLN>();
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        private XmlNamespaceManager nsManager;


        public XmlNode XmlNode
        {
            get
            {
                return xmlNode;
            }
        }

        public void DeleteLogicalNode(SclLN sclLN)
        {
            xmlNode.RemoveChild(sclLN.xmlNode);

            logicalNodes.Remove(sclLN);
        }

        public bool AddLogicalNodeOnLntargetIndex(SclLN sourceSclLN, SclLN targetSclLn)
        {
            try
            {
                logicalNodes.Remove(sourceSclLN);
                logicalNodes.Insert(logicalNodes.IndexOf(targetSclLn), sourceSclLN);

                XmlNodeList xmlNodeList = xmlNode.ChildNodes;
                foreach (XmlNode item in xmlNodeList)
                    xmlNode.RemoveChild(item);

                foreach (SclLN sclLN in logicalNodes)
                    xmlNode.AppendChild(sclLN.xmlNode);


                return true;
            }
            catch (Exception)
            {
                return false;
            }

        }

        public string Inst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "inst");

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "inst", value);
            }
        }

        public string Desc
        {
            get
            {
                return desc;
            }
        }

        public string LdName
        {
            get
            {
                return ldName;
            }

        }

        public List<SclLN> LogicalNodes
        {
            get
            {
                return logicalNodes;
            }
        }

        public SclLDevice(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.nsManager = nsManager;
            sclDocument = SclxmlDocument;
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;
            XmlAttribute instAttr = xmlNode.Attributes["inst"];

            if (instAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No inst attribute", this, "MissintInst");

            inst = instAttr.Value;

            XmlAttribute ldNameAttr = xmlNode.Attributes["ldName"];

            if (ldNameAttr != null)
                ldName = ldNameAttr.Value;

            XmlAttribute descAttr = xmlNode.Attributes["desc"];

            if (descAttr != null)
                desc = descAttr.Value;

            XmlNodeList xmlNodeList = xmlNode.ChildNodes;
            int ln0 = 0;

            if (xmlNodeList.Count > 0)
            {
                foreach (XmlNode lnNode in xmlNodeList)
                {
                    if (lnNode.Name == "LN" || lnNode.Name == "LN0")
                    {
                        if (lnNode.Name == "LN0")
                            ln0++;

                        logicalNodes.Add(new SclLN(SclxmlDocument, lnNode, nsManager));
                    }
                }

            }
            else
            {
                logicalNodes = null;
            }

            if (ln0 == 0)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "LDevice \"" + inst + "\" has no LN0", this, "lN0");

        }

    }

    public class SclFCDA
    {
        public string LdInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "ldInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "ldInst", value);
            }
        }

        public string Prefix
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "prefix");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "prefix", value);
            }
        }

        public string LnClass
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "lnClass");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "lnClass", value);
            }
        }

        public string LnInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "lnInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "lnInst", value);
            }
        }

        public string DoName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "doName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "doName", value);
            }
        }

        public string DaName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "daName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "daName", value);
            }
        }

        public SclFC Fc
        {
            get
            {
                string fcAttr = XmlHelper.GetAttributeValue(xmlNode, "fc");
                if (fcAttr != null)
                    return (SclFC)Enum.Parse(typeof(SclFC), fcAttr);
                else
                    return (SclFC.NONE);
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "fc", value.ToString());
            }
        }

        public string Index
        {
            get
            {
                string ixStr = XmlHelper.GetAttributeValue(xmlNode, "ix");

                if (ixStr != null)
                {
                    int retVal = -1;
                    Int32.TryParse(ixStr, out retVal);

                    return (retVal.ToString());
                }
                else
                    return ("");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "ix", value.ToString());
            }
        }

        internal XmlNode xmlNode = null;
        private XmlDocument xmlDocument;

        public SclFCDA(XmlNode xmlNode, XmlDocument xmlDocument)
        {
            this.xmlDocument = xmlDocument;
            this.xmlNode = xmlNode;
        }

        public string GetReferenceString()
        {
            string refStr = LdInst + "/";

            if (Prefix != null)
                refStr += Prefix;

            refStr += LnClass;

            if (LnInst != null)
                refStr += LnInst;

            if (DoName != null)
            {
                refStr += ".";
                refStr += DoName;

                if (DaName != null)
                    if (DaName != "")
                        refStr += "." + DaName;
            }

            refStr += "[" + Fc.ToString() + "]";

            return refStr;
        }

        public string GetObjectReference()
        {
            string refStr = LdInst + "/";

            if (Prefix != null)
                refStr += Prefix;

            refStr += LnClass;

            if (LnInst != null)
                refStr += LnInst;

            if (DoName != null)
            {
                refStr += ".";
                refStr += DoName;

                if (DaName != null)
                    refStr += "." + DaName;
            }

            return refStr;
        }

        public override string ToString()
        {
            return GetReferenceString();
        }
    }

    public class SclDataSet
    {
        internal XmlNode xmlNode;
        private XmlDocument xmlDocument;

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "name", value);
            }
        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "desc", value);
            }
        }

        private List<SclFCDA> fcdas = new List<SclFCDA>();

        public List<SclFCDA> Fcdas
        {
            get
            {
                return fcdas;
            }
        }

        public void Add(SclFCDA fcda)
        {
            xmlNode.AppendChild(fcda.xmlNode);
            fcdas.Add(fcda);
        }

        public void Remove(SclFCDA fcda)
        {
            xmlNode.RemoveChild(fcda.xmlNode);
            fcdas.Remove(fcda);
        }

        public void RemoveAllFcdas()
        {
            foreach (XmlNode xmlNodes in xmlNode.ChildNodes)
                xmlNode.RemoveChild(xmlNodes);

            fcdas.Clear();
        }

        public void RemoveAll()
        {
            xmlNode.RemoveAll();
            fcdas.Clear();
        }


        public SclDataSet(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            this.xmlNode = xmlNode;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "DataSet has no name attribute", this, "MissingName");


            XmlNodeList fcdaNodes = xmlNode.SelectNodes("scl:FCDA", nsManager);

            foreach (XmlNode fcdaNode in fcdaNodes)
            {
                SclFCDA sclFCDA = new SclFCDA(fcdaNode, xmlDocument);
                fcdas.Add(sclFCDA);
            }
        }
    }

    public class SclLog
    {
        internal XmlNode xmlNode;
        private XmlDocument xmlDocument;

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "name", value);
            }
        }

        public SclLog(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            this.xmlNode = xmlNode;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "Log has no name attribute", this, "MissingName");

        }
    }

    public class SclTrgOps
    {
        internal XmlNode xmlNode = null;
        private XmlDocument xmlDocument;

        public int GetIntValue()
        {
            int intValue = 0;

            if (Dchg) intValue += 1;
            if (Qchg) intValue += 2;
            if (Dupd) intValue += 4;
            if (Period) intValue += 8;
            if (GI) intValue += 16;

            return intValue;
        }

        public bool Dchg
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "dchg", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "dchg", value);
            }
        }

        public bool Qchg
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "qchg", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "qchg", value);
            }
        }

        public bool Dupd
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "dupd", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "dupd", value);
            }
        }

        public bool Period
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "period", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "period", value);
            }
        }

        public bool GI
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "gi", true);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "gi", value);
            }
        }

        public SclTrgOps(XmlDocument xmlDoc, XmlNode xmlNode)
        {
            xmlDocument = xmlDoc;
            this.xmlNode = xmlNode;
        }
    }

    public class SclOptFields
    {
        internal XmlNode xmlNode = null;
        private XmlDocument xmlDocument;

        public int GetIntValue()
        {
            int intValue = 0;

            if (SeqNum) intValue += 1;
            if (TimeStamp) intValue += 2;
            if (ReasonCode) intValue += 4;
            if (DataSet) intValue += 8;
            if (DataRef) intValue += 16;
            if (BufOvfl) intValue += 32;
            if (EntryID) intValue += 64;
            if (ConfigRef) intValue += 128;

            return intValue;
        }

        public bool SeqNum
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "seqNum", false); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "seqNum", value);
            }
        }

        public bool TimeStamp
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "timeStamp", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "timeStamp", value);
            }
        }

        public bool DataSet
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "dataSet", false); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "dataSet", value);
            }
        }

        public bool ReasonCode
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "reasonCode", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "reasonCode", value);
            }
        }

        public bool DataRef
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "dataRef", false); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "dataRef", value);
            }
        }

        public bool EntryID
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "entryID", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "entryID", value);
            }
        }

        public bool ConfigRef
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "configRef", false); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "configRef", value);
            }
        }

        public bool BufOvfl
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "bufOvfl", true); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "bufOvfl", value);
            }
        }


        public SclOptFields(XmlDocument xmlDocument, XmlNode xmlNode)
        {
            this.xmlNode = xmlNode;
            this.xmlDocument = xmlDocument;
        }
    }

    public class SclClientLN
    {
        private string iedName = null;
        private string apRef = null;
        private string ldInst = null;
        private string prefix = null;
        private string lnClass = null;
        private string lnInst = null;
        private string desc = null;
        private XmlDocument xmlDoc;

        public string IedName
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "iedName");

                if (valStr != null)
                    return valStr;
                else
                    return null;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "iedName", value.ToString());
            }
        }

        public string ApRef
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "apRef");

                if (valStr != null)
                    return valStr;
                else
                    return null;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "apRef", value.ToString());
            }
        }

        public string LdInst
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "ldInst");

                if (valStr != null)
                    return valStr;
                else
                    return null;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "ldInst", value.ToString());
            }
        }

        public string Prefix
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "prefix");

                if (valStr != null)
                    return valStr;
                else
                    return null;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "prefix", value.ToString());
            }
        }

        public string LnClass
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "lnClass");

                if (valStr != null)
                    return valStr;
                else
                    return null;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "lnClass", value.ToString());
            }
        }

        public string LnInst
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "lnInst");

                if (valStr != null)
                    return valStr;
                else
                    return null;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "lnInst", value.ToString());
            }
        }

        public string Desc
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "desc");

                if (valStr != null)
                    return valStr;
                else
                    return null;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "desc", value.ToString());
            }
        }

        public XmlNode xmlNode = null;

        public SclClientLN(XmlNode xmlNode, XmlDocument xmlDoc)
        {
            this.xmlNode = xmlNode;
            this.xmlDoc = xmlDoc;

            XmlAttribute iedNameAttr = xmlNode.Attributes["iedName"];

            if (iedNameAttr != null)
                iedName = iedNameAttr.Value;

            XmlAttribute apRefAttr = xmlNode.Attributes["apRef"];

            if (apRefAttr != null)
                apRef = apRefAttr.Value;

            XmlAttribute ldInstAttr = xmlNode.Attributes["ldInst"];

            if (ldInstAttr != null)
                ldInst = ldInstAttr.Value;

            XmlAttribute prefixAttr = xmlNode.Attributes["prefix"];

            if (prefixAttr != null)
                prefix = prefixAttr.Value;

            XmlAttribute lnClassAttr = xmlNode.Attributes["lnClass"];

            if (lnClassAttr != null)
                lnClass = lnClassAttr.Value;

            XmlAttribute lnInstAttr = xmlNode.Attributes["lnInst"];

            if (lnInstAttr != null)
                lnInst = lnInstAttr.Value;

            XmlAttribute descAttr = xmlNode.Attributes["desc"];

            if (descAttr != null)
                desc = descAttr.Value;
        }

        public string GetReferenceString()
        {
            string refStr = "";

            if (apRef != null)
                refStr += apRef + ":";

            if (iedName != null)
                refStr += iedName + "/";

            if (prefix != null)
                refStr += prefix;

            refStr += lnClass;

            if (lnInst != null)
                refStr += lnInst;

            return refStr;
        }
    }

    public class SclRptEnabled
    {
        public XmlNode xmlNode = null;
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;


        private List<SclClientLN> clientLNs = new List<SclClientLN>();

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "desc", value);
            }
        }

        public int Max
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "max");

                if (valStr != null)
                {
                    int retVal = 1;
                    Int32.TryParse(valStr, out retVal);

                    return (retVal);
                }
                else
                    return 1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "max", value.ToString());
            }
        }

        public void AddClientLN(SclClientLN clientLn)
        {
            xmlNode.AppendChild(clientLn.xmlNode);
            clientLNs.Add(clientLn);
        }


        public void RemoveAllClientLNs()
        {
            foreach (XmlNode xmlNodes in xmlNode.ChildNodes)
                xmlNode.RemoveChild(xmlNodes);

            clientLNs.Clear();
        }

        public void RemoveClientLN(SclClientLN sclClientLN)
        {
            xmlNode.RemoveChild(sclClientLN.xmlNode);
            clientLNs.Remove(sclClientLN);
        }

        public List<SclClientLN> ClientLNs
        {
            get
            {
                return clientLNs;
            }
        }

        public SclRptEnabled(XmlNode xmlNode, XmlDocument xmlDoc, XmlNamespaceManager nsManager)
        {
            this.xmlNode = xmlNode;
            this.xmlDoc = xmlDoc;
            this.nsManager = nsManager;

            XmlNodeList clientLNNodes = xmlNode.SelectNodes("scl:ClientLN", nsManager);

            if (clientLNNodes != null)
            {
                foreach (XmlNode clientLNNode in clientLNNodes)
                {
                    SclClientLN clientLN = new SclClientLN(clientLNNode, xmlDoc);

                    clientLNs.Add(clientLN);
                }
            }

        }

    }

    public class SclReportControl
    {
        private string name = null;
        private string desc = null;
        private string datSet = null;
        private int intgPd = -1;
        private string rptID = null;
        private long confRev = -1;
        private bool buffered = false;
        private int bufTime = 0;
        private bool indexed = true;

        private SclTrgOps trgOps = null;
        private SclOptFields optFields = null;
        private SclRptEnabled rptEna = null;

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "name", value);
            }
        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "desc", value);
            }
        }

        public string DatSet
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "datSet");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "datSet", value);
            }
        }

        public string IntgPd
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "intgPd");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "intgPd", value);
            }
        }

        public string RptID
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "rptID");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "rptID", value);
            }
        }

        public string ConfRev
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "confRev");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "confRev", value);
            }
        }

        public int IntConfRev
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "confRev");

                if (valStr != null)
                {
                    int retVal = -1;
                    Int32.TryParse(valStr, out retVal);

                    return (retVal);
                }
                else
                    return (-1);
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "confRev", value.ToString());
            }
        }

        public bool Buffered
        {
            get
            {
                string result = XmlHelper.GetAttributeValue(xmlNode, "buffered");

                if (result == "true")
                    return true;
                else
                    return false;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "buffered", value);
            }
        }

        public string BufTime
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "bufTime");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "bufTime", value);
            }
        }

        public bool Indexed
        {
            get
            {

                string result = XmlHelper.GetAttributeValue(xmlNode, "indexed");

                if (result == null)
                    return true;

                if (result == "true")
                    return true;
                else
                    return false;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "indexed", value);
            }
        }

        public SclTrgOps TrgOps
        {
            get
            {
                return trgOps;
            }
        }

        public SclOptFields OptFields
        {
            get
            {
                return optFields;
            }
        }

        public SclRptEnabled RptEna
        {
            get
            {
                return rptEna;
            }
        }

        public XmlNode xmlNode = null;
        private XmlDocument xmlDocument;

        private bool ParseBooleanAttribute(string attributeName, bool defaultValue)
        {
            XmlAttribute attr = xmlNode.Attributes[attributeName];

            bool attrVal = defaultValue;

            if (attr != null)
            {
                if (Boolean.TryParse(attr.Value, out attrVal) == false)
                    throw new SclParserException(xmlNode, "ReportControl: failed to parse boolean attribute");
            }

            return attrVal;
        }

        public SclReportControl(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;

            indexed = ParseBooleanAttribute("indexed", indexed);

            XmlAttribute bufTimeAttr = xmlNode.Attributes["bufTime"];

            if (bufTimeAttr != null)
            {
                if (int.TryParse(bufTimeAttr.Value, out bufTime) == false)
                    SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "ReportControl: failed to parse \"bufTime\" attribute", this, "BufTimeFailed");
            }

            buffered = ParseBooleanAttribute("buffered", buffered);

            XmlAttribute confRevAttr = xmlNode.Attributes["confRev"];

            if (confRevAttr != null)
            {
                if (long.TryParse(confRevAttr.Value, out confRev) == false)
                    SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "ReportControl: failed to parse \"confRev\" attribute", this, "ConfRevFailed");
            }

            XmlAttribute rptIDAttr = xmlNode.Attributes["rptID"];

            if (rptIDAttr != null)
                rptID = rptIDAttr.Value;

            XmlAttribute intgPdAttr = xmlNode.Attributes["intgPd"];

            if (intgPdAttr != null)
            {
                if (int.TryParse(intgPdAttr.Value, out intgPd) == false)
                    SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "ReportControl: failed to parse \"intgPd\" attribute", this, "IntgPdFailed");
            }

            XmlAttribute datSetAttr = xmlNode.Attributes["datSet"];

            if (datSetAttr != null)
                datSet = datSetAttr.Value;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr != null)
            {
                name = nameAttr.Value;
            }
            else
            {
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "ReportControl has no name attribute", this, "MissingName");
            }


            XmlAttribute descAttr = xmlNode.Attributes["desc"];

            if (descAttr != null)
                desc = descAttr.Value;

            XmlNode trgOpsNode = xmlNode.SelectSingleNode("scl:TrgOps", nsManager);

            if (trgOpsNode != null)
                trgOps = new SclTrgOps(xmlDocument, trgOpsNode);

            XmlNode optFieldsNode = xmlNode.SelectSingleNode("scl:OptFields", nsManager);

            if (optFieldsNode != null)
                optFields = new SclOptFields(xmlDocument, optFieldsNode);

            XmlNode rptEnaNode = xmlNode.SelectSingleNode("scl:RptEnabled", nsManager);

            if (rptEnaNode != null)
                rptEna = new SclRptEnabled(rptEnaNode, xmlDocument, nsManager);
        }
    }

    public class SclDAI
    {
        private XmlNode xmlNode;
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;
        private object parent;
        private List<SclVal> values = new List<SclVal>();

        public List<SclVal> GetValues()
        {
            return values;
        }

        public object Parent { get { return parent; } set { parent = value; } }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "desc", value);
            }
        }

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "name", value);
            }

        }

        public string SAddr
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "sAddr");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "sAddr", value);
            }
        }

        public string ValKind
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "valKind");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "valKind", value);
            }
        }

        public string ValImport
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "valImport");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "valImport", value);
            }
        }

        public string Ix
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "ix");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "ix", value);
            }
        }

        public string Val
        {
            get
            {
                XmlNode VALNode = null;

                if (nsManager != null)
                    VALNode = xmlNode.SelectSingleNode("scl:Val", nsManager);
                else
                    VALNode = xmlNode.SelectSingleNode("Val");

                if (VALNode != null)
                    return VALNode.InnerText;
                else
                    return null;
            }
        }

        public XmlNode Node { get { return xmlNode; } set { xmlNode = value; } }

        public XmlDocument XmlDoc { get { return xmlDoc; } set { xmlDoc = value; } }

        public XmlNamespaceManager NsManager { get { return nsManager; } set { nsManager = value; } }


        public SclDAI(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager, string objRef = null)
        {
            xmlDoc = SclxmlDocument.XmlDocument;
            this.xmlNode = xmlNode;
            this.nsManager = nsManager;

            XmlAttribute DAIname = xmlNode.Attributes["name"];

            if (DAIname == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "DAI has no name attribute", this, "MissingName");

            if (objRef != null && DAIname != null)
                objRef += "." + DAIname.Value;

            XmlNodeList valNodes = xmlNode.SelectNodes("scl:Val", nsManager);
            if (valNodes.Count == 0)
            {
                //TODO
                //Only show warninf when there is no value and no sAddr
                //if (DAIname != null)
                //{
                //    if(objRef != null)
                //        SclxmlDocument.AddIssue(xmlNode, "WARNING", "Model integrity", "DAI " + objRef + " has no value attribute", this, "MissingValue");
                //    else
                //        SclxmlDocument.AddIssue(xmlNode, "WARNING", "Model integrity", "DAI " + DAIname.Value + " has no value attribute", this, "MissingValue");

                //}
                //else
                //    SclxmlDocument.AddIssue(xmlNode, "WARNING", "Model integrity", "DAI has no value attribute", this, "MissingValue");
            }
            else
            {
                foreach (XmlNode valNode in valNodes)
                    values.Add(new SclVal(xmlDoc, valNode));
            }
        }
    }

    public class SclDOI
    {
        private XmlNode xmlNode;
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;

        private List<SclDAI> sclDAIs = new List<SclDAI>();
        private List<SclSDI> sclSDIs = new List<SclSDI>();

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "desc", value);
            }
        }

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "name", value);
            }
        }

        public string AccessControl
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "accessControl");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "accessControl", value);
            }
        }

        public List<SclDAI> SclDAIs { get { return sclDAIs; } set { sclDAIs = value; } }

        public List<SclSDI> SclSDIs { get { return sclSDIs; } set { sclSDIs = value; } }

        public string Ix
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "ix");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "ix", value);
            }
        }


        public SclDOI(SclDocument SclxmlDoc, XmlNode xmlNode, XmlNamespaceManager nsManager, string objRef = null)
        {
            xmlDoc = SclxmlDoc.XmlDocument;
            this.xmlNode = xmlNode;
            this.nsManager = nsManager;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr == null)
                SclxmlDoc.AddIssue(xmlNode, "ERROR", "Model integrity", "DOI has no name attribute", this, "MissingName");

            if (nameAttr != null)
            {
                if (objRef != null)
                    objRef += nameAttr.Value;
                else
                    objRef = nameAttr.Value;
            }

            XmlNodeList DAINodes = xmlNode.SelectNodes("scl:DAI", nsManager);
            if (DAINodes != null)
            {
                foreach (XmlNode DAINode in DAINodes)
                {
                    SclDAI sclDAI = new SclDAI(SclxmlDoc, DAINode, nsManager, objRef);
                    sclDAI.Parent = this;
                    sclDAIs.Add(sclDAI);
                }
            }


            XmlNodeList SDINodes = xmlNode.SelectNodes("scl:SDI", nsManager);

            if (SDINodes != null)
            {
                foreach (XmlNode SDINode in SDINodes)
                {
                    SclSDI SclSDI = new SclSDI(SclxmlDoc, SDINode, nsManager, objRef);
                    SclSDI.Parent = this;
                    sclSDIs.Add(SclSDI);
                }
            }

        }

        public XmlNode Node { get { return xmlNode; } set { xmlNode = value; } }

        public XmlDocument XmlDoc { get { return xmlDoc; } set { xmlDoc = value; } }

        public XmlNamespaceManager NsManager { get { return nsManager; } set { nsManager = value; } }
    }

    public class SclSDI
    {
        private XmlNode xmlNode;
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;
        private object parent;

        public object Parent { get { return parent; } set { parent = value; } }


        private List<SclDAI> sclDAIs = new List<SclDAI>();
        private List<SclSDI> sclSDIs = new List<SclSDI>();

        public List<SclSDI> SclSDIs { get { return sclSDIs; } set { sclSDIs = value; } }
        public List<SclDAI> SclDAIs { get { return sclDAIs; } set { sclDAIs = value; } }

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "name", value);
            }

        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "desc", value);
            }

        }

        public string Ix
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "ix");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "ix", value);
            }

        }

        public string SAddr
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "sAddr");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "sAddr", value);
            }

        }

        public XmlNode Node { get { return xmlNode; } set { xmlNode = value; } }

        public XmlDocument XmlDoc { get { return xmlDoc; } set { xmlDoc = value; } }

        public XmlNamespaceManager NsManager { get { return nsManager; } set { nsManager = value; } }

        public SclSDI(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager, string objref = null)
        {
            xmlDoc = SclxmlDocument.XmlDocument;
            this.xmlNode = xmlNode;
            this.nsManager = nsManager;

            XmlAttribute SDIname = xmlNode.Attributes["name"];

            if (SDIname == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "SDI has no name attribute", this, "MissingName");

            if (SDIname != null && objref != null)
                objref += "." + SDIname.Value;


            XmlNodeList SDINodes = xmlNode.SelectNodes("scl:SDI", nsManager);

            if (SDINodes != null)
            {
                foreach (XmlNode SDINode in SDINodes)
                {
                    SclSDI SclSDI = new SclSDI(SclxmlDocument, SDINode, nsManager, objref);
                    SclSDI.Parent = this;
                    sclSDIs.Add(SclSDI);
                }
            }

            XmlNodeList DAINodes = xmlNode.SelectNodes("scl:DAI", nsManager);
            if (DAINodes != null)
            {
                foreach (XmlNode DAINode in DAINodes)
                {
                    SclDAI SclDAI = new SclDAI(SclxmlDocument, DAINode, nsManager, objref);
                    SclDAI.Parent = this;
                    sclDAIs.Add(SclDAI);
                }
                //sclDAIs.Add(new SclDAI(xmlDocument, DAINode, nsManager));
            }

        }

    }

    public class Inputs
    {
        internal XmlNode xmlNode;
        public XmlDocument xmlDocument;
        public IEDModelNode Parent;
        private List<SclExtRef> extRefs = new List<SclExtRef>();

        public List<SclExtRef> ExtRefs
        {
            get { return extRefs; }
        }

        public void Add(SclExtRef extRef)
        {
            xmlNode.AppendChild(extRef.xmlNode);
            extRef.Parent = this;
            extRefs.Add(extRef);
        }

        public void RemoveAllExtRef()
        {
            foreach (XmlNode xmlNodes in xmlNode.ChildNodes)
                xmlNode.RemoveChild(xmlNodes);

            extRefs.Clear();
        }

        public void Remove(SclExtRef extRef)
        {
            xmlNode.RemoveChild(extRef.xmlNode);
            extRefs.Remove(extRef);
        }

        public Inputs(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;

            XmlNodeList extRefNodes = xmlNode.SelectNodes("scl:ExtRef", nsManager);
            foreach (XmlNode extRefNode in extRefNodes)
                extRefs.Add(new SclExtRef(SclxmlDocument, extRefNode));
        }


        public SclExtRef AddExtRef(SclExtRef extRef)
        {
            xmlNode.AppendChild(extRef.xmlNode);
            extRef.Parent = this;
            extRefs.Add(extRef);

            return extRef;
        }


    }

    public class SclLN
    {
        private string lnClass;
        private string lnType;
        private string inst;
        private string desc = null;
        private string prefix = null;


        internal XmlNode xmlNode;
        public XmlDocument xmlDocument;
        private SclDocument sclDocument;

        private Inputs inputs = null;
        private SclSettingControl settingControl = null;
        private List<SclDataSet> dataSets = new List<SclDataSet>();
        private List<SclLog> logs = new List<SclLog>();
        private List<SclReportControl> reportControls = new List<SclReportControl>();
        private List<SclGSEControl> gseControls = new List<SclGSEControl>();
        private List<SclLogControl> logControls = new List<SclLogControl>();
        private List<SclSMVControl> sclSMVControls = new List<SclSMVControl>();
        private List<SclDOI> dois = new List<SclDOI>();

        public XmlNode XmlNode
        {
            get
            {
                return xmlNode;
            }
        }


        public Inputs Inputs
        {
            get { return inputs; }
            set { inputs = value; }
        }

        public SclSettingControl SettingControl
        {
            get { return settingControl; }
            set { settingControl = value; }
        }

        public List<SclDataSet> DataSets
        {
            get
            {
                return dataSets;
            }
        }

        public List<SclLog> Logs
        {
            get
            {
                return logs;
            }
        }

        public List<SclDOI> DOIs
        {
            get
            {
                return dois;
            }
        }

        public List<SclReportControl> ReportControls
        {
            get
            {
                return reportControls;
            }
        }

        public List<SclGSEControl> GSEControls
        {
            get
            {
                return gseControls;
            }
        }

        public List<SclSMVControl> SclSMVControls
        {
            get
            {
                return sclSMVControls;
            }
        }

        public List<SclLogControl> LogControls
        {
            get
            {
                return logControls;
            }
        }

        public string InstanceName
        {
            get
            {
                return Prefix + LnClass + Inst;

            }

        }

        public string LnClass
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "lnClass");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "lnClass", value);
            }
        }

        public string LnType
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "lnType");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "lnType", value);
            }
        }

        public string Inst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "inst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "inst", value);
            }
        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "desc", value);
            }
        }

        public string Prefix
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "prefix");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "prefix", value);
            }
        }

        private XmlNamespaceManager nsManager;


        public SclLN(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;
            this.nsManager = nsManager;
            sclDocument = SclxmlDocument;

            XmlAttribute lnClassAttr = xmlNode.Attributes["lnClass"];

            if (lnClassAttr == null)
            {
                lnClass = null;
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "no lnClass attribute", this, "lnClass");
            }
            else
            {
                lnClass = lnClassAttr.Value;

            }

            XmlAttribute lnTypeAttr = xmlNode.Attributes["lnType"];

            if (lnTypeAttr == null)
            {
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "no lnType attribute", this, "lnType");
                lnType = null;
            }
            else
                lnType = lnTypeAttr.Value;


            XmlAttribute instAttr = xmlNode.Attributes["inst"];

            if (instAttr == null)
            {
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "no inst attribute", this, "inst");
                inst = null;
            }
            else
                inst = instAttr.Value;

            XmlAttribute descAttr = xmlNode.Attributes["desc"];

            if (descAttr != null)
                desc = descAttr.Value;

            XmlAttribute prefixAttr = xmlNode.Attributes["prefix"];

            if (prefixAttr != null)
                prefix = prefixAttr.Value;

            XmlNodeList dataSetNodes = xmlNode.SelectNodes("scl:DataSet", nsManager);

            foreach (XmlNode dataSetNode in dataSetNodes)
                dataSets.Add(new SclDataSet(SclxmlDocument, dataSetNode, nsManager));

            XmlNodeList reportControlNodes = xmlNode.SelectNodes("scl:ReportControl", nsManager);

            foreach (XmlNode reportControlNode in reportControlNodes)
                reportControls.Add(new SclReportControl(SclxmlDocument, reportControlNode, nsManager));

            XmlNodeList gseControlNodes = xmlNode.SelectNodes("scl:GSEControl", nsManager);

            foreach (XmlNode gseControlNode in gseControlNodes)
                gseControls.Add(new SclGSEControl(SclxmlDocument, gseControlNode));

            XmlNodeList smvControlNodes = xmlNode.SelectNodes("scl:SampledValueControl", nsManager);

            foreach (XmlNode smvControlNode in smvControlNodes)
                sclSMVControls.Add(new SclSMVControl(SclxmlDocument, smvControlNode, nsManager));

            XmlNodeList logControlNodes = xmlNode.SelectNodes("scl:LogControl", nsManager);

            foreach (XmlNode LogControlNode in logControlNodes)
                logControls.Add(new SclLogControl(SclxmlDocument, LogControlNode, nsManager));

            XmlNodeList DOINodes = xmlNode.SelectNodes("scl:DOI", nsManager);

            foreach (XmlNode DOINode in DOINodes)
                dois.Add(new SclDOI(SclxmlDocument, DOINode, nsManager));

            XmlNode inputs = xmlNode.SelectSingleNode("scl:Inputs", nsManager);

            if (inputs != null)
                Inputs = new Inputs(SclxmlDocument, inputs, nsManager);

            XmlNode settingControlNode = xmlNode.SelectSingleNode("scl:SettingControl", nsManager);

            if (settingControlNode != null)
                settingControl = new SclSettingControl(SclxmlDocument, settingControlNode);

            XmlNodeList logNodes = xmlNode.SelectNodes("scl:Log", nsManager);

            foreach (XmlNode logNode in logNodes)
                logs.Add(new SclLog(SclxmlDocument, logNode, nsManager));

        }

    }
}

