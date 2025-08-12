/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System;
using System.Collections.Generic;
using System.Xml;

namespace IEC61850.SCL
{
    /// <summary>
	/// Representation of the SCL Header.History.Hitem (history item) element
    /// </summary>
    public class SclHitem
    {
        public XmlNode XmlNode = null;
        private XmlDocument xmlDocument;

        public string Version
        {
            get
            {
                return XmlHelper.GetAttributeValue(XmlNode, "version");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, XmlNode, "version", value);
            }
        }

        public string Revision
        {
            get
            {
                return XmlHelper.GetAttributeValue(XmlNode, "revision");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, XmlNode, "revision", value);
            }
        }

        public string What
        {
            get
            {
                return XmlHelper.GetAttributeValue(XmlNode, "what");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, XmlNode, "what", value);
            }
        }

        public string Who
        {
            get
            {
                return XmlHelper.GetAttributeValue(XmlNode, "who");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, XmlNode, "who", value);
            }
        }

        public string When
        {
            get
            {
                return XmlHelper.GetAttributeValue(XmlNode, "when");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, XmlNode, "when", value);
            }
        }

        public string Why
        {
            get
            {
                return XmlHelper.GetAttributeValue(XmlNode, "why");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, XmlNode, "why", value);
            }
        }

        public SclHitem(SclDocument SclxmlDocument, XmlNode xmlNode)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            XmlNode = xmlNode;

            XmlAttribute version = xmlNode.Attributes["version"];
            if (version == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No version attribute on Hitem", this, "version");

            XmlAttribute revision = xmlNode.Attributes["revision"];
            if (revision == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No revision attribute on Hitem", this, "revision");

            XmlAttribute when = xmlNode.Attributes["when"];
            if (when == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No when attribute on Hitem", this, "when");
        }

        public SclHitem(XmlDocument xmlDocument)
        {
            this.xmlDocument = xmlDocument;
            XmlNode = xmlDocument.CreateElement("Hitem", SclDocument.SCL_XMLNS);

            Version = "";
            Revision = "";
            When = "";
        }
    }

    public class SclHistory
    {
        private XmlNode xmlNode = null;
        private XmlDocument xmlDocument;

        private List<SclHitem> hitems = new List<SclHitem>();

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public List<SclHitem> HItems
        {
            get
            {
                return hitems;
            }
        }

        public SclHitem AddHitem()
        {
            SclHitem sclHitem = new SclHitem(xmlDocument);
            hitems.Add(sclHitem);

            xmlNode.AppendChild(sclHitem.XmlNode);

            return sclHitem;
        }

        public bool DeleteHitem(SclHitem sclHitem)
        {
            try
            {
                hitems.Remove(sclHitem);

                xmlNode.RemoveChild(sclHitem.XmlNode);

                return true;
            }
            catch (Exception)
            {
                return false;
            }




        }

        public SclHistory(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            this.xmlNode = xmlNode;

            XmlNodeList hitemNodes = xmlNode.SelectNodes("scl:Hitem", nsManager);

            foreach (XmlNode hitemNode in hitemNodes)
            {
                hitems.Add(new SclHitem(SclxmlDocument, hitemNode));
            }
        }

        public SclHistory(XmlDocument xmlDocument)
        {
            this.xmlDocument = xmlDocument;
            xmlNode = xmlDocument.CreateElement("History", SclDocument.SCL_XMLNS);

            SclHitem sclHitem = new SclHitem(xmlDocument);
            xmlNode.AppendChild(sclHitem.XmlNode);
            hitems.Add(sclHitem);
        }
    }

    public class SclHeader
    {
        private XmlNode xmlNode = null;
        private XmlDocument xmlDocument;

        private SclHistory history = null;

        public string MyProperty
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "id");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "id", value);
            }
        }

        public XmlNode XmlNode
        {
            get
            {
                return xmlNode;
            }
        }

        public string Version
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "version");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "version", value);
            }
        }

        public string Revision
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "revision");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "revision", value);
            }
        }

        public string ToolID
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "toolID");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "toolID", value);
            }
        }

        public string NameStructure
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "nameStructure");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "nameStructure", value);
            }
        }

        public SclHistory History
        {
            get
            {
                return history;
            }
        }

        public bool DeleteHistory()
        {
            if (history != null)
            {
                xmlNode.RemoveChild(history.XmlNode);
                history = null;

                return true;
            }

            return false;
        }

        public XmlNode Node
        {
            get { return xmlNode; }
        }

        public SclHistory AddHistory()
        {
            if (history == null)
            {
                history = new SclHistory(xmlDocument);
                xmlNode.AppendChild(history.XmlNode);
                return history;
            }
            else
                return null;
        }

        public SclHeader(SclDocument SclxmlDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclxmlDocument.XmlDocument;
            this.xmlNode = xmlNode;

            XmlAttribute idAttr = xmlNode.Attributes["id"];

            if (idAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No id attribute on Header", this, "id");


            XmlNode historyNode = xmlNode.SelectSingleNode("scl:History", nsManager);

            if (historyNode != null)
                history = new SclHistory(SclxmlDocument, historyNode, nsManager);
        }

        public SclHeader(SclDocument SclDocument, XmlNamespaceManager nsManager)
        {
            xmlDocument = SclDocument.XmlDocument;
            xmlNode = SclDocument.XmlDocument.CreateElement("Header", SclDocument.SCL_XMLNS);

            MyProperty = "";

            history = new SclHistory(SclDocument.XmlDocument);
            xmlNode.AppendChild(history.XmlNode);
        }
    }
}
