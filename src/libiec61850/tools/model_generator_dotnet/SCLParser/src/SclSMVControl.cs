/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System;
using System.Xml;

namespace IEC61850.SCL
{
    public class SclSMVControl
    {
        internal XmlNode xmlNode;
        private XmlDocument xmlDocument;
        private SclSmvOpts smvOpts = null;

        public SclSMVControl(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "SMVControl has no name attribute", this, "MissingName");

            XmlNode smvOptsNode = xmlNode.SelectSingleNode("scl:SmvOpts", nsManager);

            if (smvOptsNode != null)
                smvOpts = new SclSmvOpts(xmlDocument, smvOptsNode);
        }


        public SclSmvOpts SclSmvOpts
        {
            get
            {
                return smvOpts;
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

        public string DataSet
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


        public int ConfRev
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

        public string SmvID
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "smvID");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "smvID", value);
            }
        }

        public bool Multicast
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "multicast", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "multicast", value);
            }
        }

        public int SmpRate
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "smpRate");

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
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "smpRate", value.ToString());
            }

        }

        public int NofASDU
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "nofASDU");

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
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "nofASDU", value.ToString());
            }
        }

        public string SmpMod
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "smpMod");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "smpMod", value);
            }
        }

        public string SecurityEnabled
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "securityEnabled");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "securityEnabled", value);
            }
        }

    }

    public class SclSmvOpts
    {
        internal XmlNode xmlNode = null;
        private XmlDocument xmlDocument;

        public bool RefreshTime
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "refreshTime", false); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "refreshTime", value);
            }
        }

        public bool SampleRate
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "sampleRate", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "sampleRate", value);
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

        public bool Security
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "security", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "security", value);
            }
        }

        public bool SynchSourceId
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "synchSourceId", false); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "synchSourceId", value);
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

        public bool SampleSynchronized
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "sampleSynchronized", false); ;
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "sampleSynchronized", value);
            }
        }

        public int GetIntValue()
        {
            int intValue = 0;

            if (RefreshTime) intValue += 1;
            if (SampleSynchronized) intValue += 2;
            if (SampleRate) intValue += 4;
            if (DataSet) intValue += 8;
            if (Security) intValue += 16;

            return intValue;
        }


        public SclSmvOpts(XmlDocument xmlDocument, XmlNode xmlNode)
        {
            this.xmlNode = xmlNode;
            this.xmlDocument = xmlDocument;
        }
    }

}
