/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System.Xml;

namespace IEC61850.SCL
{
    public class SclLogControl
    {
        public XmlNode xmlNode;
        private XmlDocument xmlDocument;
        private SclTrgOps trgOps = null;

        public SclTrgOps TrgOps
        {
            get
            {
                return trgOps;
            }
        }


        public SclLogControl(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "LogControl has no name attribute", this, "MissingName");

            XmlNode trgOpsNode = xmlNode.SelectSingleNode("scl:TrgOps", nsManager);

            if (trgOpsNode != null)
                trgOps = new SclTrgOps(xmlDocument, trgOpsNode);
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

        public string LogName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "logName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "logName", value);
            }
        }

        public bool LogEna
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "logEna", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "logEna", value);
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

    }
}
