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
    public class SclEquipment : SclPowerSystemResource
    {
        public bool Virtual
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "virtual", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "virtual", value);
            }
        }

        internal SclEquipment(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {

        }
    }

    public class SclSubEquipment : SclPowerSystemResource
    {
        public bool Virtual
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "virtual", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "virtual", value);
            }
        }

        public string Phase
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "phase");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "phase", value);
            }
        }

        private List<SclEqFunction> sclEqFunctions;

        private void parseEqFunctions()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:EqFunction", nsManager);

            sclEqFunctions = new List<SclEqFunction>();

            foreach (XmlNode node in nodes)
            {
                SclEqFunction lNode = new SclEqFunction(xmlDocument, sclDocument, node, nsManager);
                sclEqFunctions.Add(lNode);
            }
        }

        public List<SclEqFunction> EqFunctions
        {
            get { return sclEqFunctions; }
        }

        public bool RemoveEqFunction(SclEqFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                EqFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public SclEqFunction AddNewEqFunction()
        {
            SclEqFunction newControl = new SclEqFunction(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (EqFunctions.Count > 0)
            {
                int lastIndex = EqFunctions.Count - 1;

                SclEqFunction lastLNode = EqFunctions[lastIndex];

                XmlNode parent = lastLNode.xmlNode.ParentNode;

                parent.InsertAfter(newNode, lastLNode.xmlNode);
            }
            else
            {
                XmlNode parent = XmlNode;
                parent.AppendChild(newNode);

            }

            try
            {
                newControl = new SclEqFunction(xmlDocument, sclDocument, newNode, nsManager);
                EqFunctions.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
            }
        }

        internal SclSubEquipment(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseEqFunctions();
        }

        public SclSubEquipment(XmlDocument xmlDocument, SclDocument sclDocument, XmlNamespaceManager nsManager)
                : base(xmlDocument, sclDocument, xmlDocument.CreateElement("SubEquipment", SclDocument.SCL_XMLNS), nsManager)
        {

        }
    }

    public class SclAbstractConductingEquipment : SclEquipment
    {
        private List<SclTerminal> sclTerminals;
        private List<SclSubEquipment> sclSubEquipments;
        private void parseTerminal()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:Terminal", nsManager);

            sclTerminals = new List<SclTerminal>();

            foreach (XmlNode node in nodes)
            {
                SclTerminal lNode = new SclTerminal(xmlDocument, sclDocument, node, nsManager);
                sclTerminals.Add(lNode);
            }
        }

        private void parseSubEquipment()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:SubEquipment", nsManager);

            sclSubEquipments = new List<SclSubEquipment>();

            foreach (XmlNode node in nodes)
            {
                SclSubEquipment lNode = new SclSubEquipment(xmlDocument, sclDocument, node, nsManager);
                sclSubEquipments.Add(lNode);
            }
        }

        public List<SclTerminal> Terminals
        {
            get
            {
                return sclTerminals;
            }
        }

        public bool RemoveTerminal(SclTerminal node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                Terminals.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public SclTerminal AddNewTerminal()
        {
            SclTerminal newControl = new SclTerminal(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (Terminals.Count > 0)
            {
                int lastIndex = Terminals.Count - 1;

                SclTerminal lastLNode = Terminals[lastIndex];

                XmlNode parent = lastLNode.xmlNode.ParentNode;

                parent.InsertAfter(newNode, lastLNode.xmlNode);
            }
            else
            {
                XmlNode parent = XmlNode;
                parent.AppendChild(newNode);

            }

            try
            {
                newControl = new SclTerminal(xmlDocument, sclDocument, newNode, nsManager);
                Terminals.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
            }
        }

        public List<SclSubEquipment> SubEquipments
        {
            get
            {
                return sclSubEquipments;
            }
        }

        public bool RemoveSubEquipment(SclSubEquipment node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                SubEquipments.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public SclSubEquipment AddNewSubEquipment()
        {
            SclSubEquipment newControl = new SclSubEquipment(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (SubEquipments.Count > 0)
            {
                int lastIndex = SubEquipments.Count - 1;

                SclSubEquipment lastLNode = SubEquipments[lastIndex];

                XmlNode parent = lastLNode.xmlNode.ParentNode;

                parent.InsertAfter(newNode, lastLNode.xmlNode);
            }
            else
            {
                XmlNode parent = XmlNode;
                parent.AppendChild(newNode);

            }

            try
            {
                newControl = new SclSubEquipment(xmlDocument, sclDocument, newNode, nsManager);
                SubEquipments.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
            }
        }

        internal SclAbstractConductingEquipment(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseTerminal();

            if (sclTerminals.Count > 2)
            {
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "AbstractConductingEquipment contains " + sclTerminals.Count + " Terminals. Maximum  is 2", this, "Terminal");
            }

            parseSubEquipment();
        }
    }

    public class SclTerminal : SclBaseElement
    {
        private XmlDocument xmlDocument;

        public string BayName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "bayName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "bayName", value);
            }
        }

        public string CNodeName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "cNodeName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "cNodeName", value);
            }
        }

        public string ConnectivityNode
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "connectivityNode");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "connectivityNode", value);
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

        public string NeutralPoint
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "neutralPoint");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "neutralPoint", value);
            }
        }

        public string PorcessName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "porcessName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "porcessName", value);
            }
        }

        public string SubstationName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "substationName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "substationName", value);
            }
        }

        public string VoltageLevelName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "voltageLevelName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "voltageLevelName", value);
            }
        }

        internal SclTerminal(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode)
        {
            this.xmlDocument = xmlDocument;

        }

        public SclTerminal(XmlDocument xmlDocument, SclDocument sclDocument, XmlNamespaceManager nsManager)
                 : base(xmlDocument, sclDocument, xmlDocument.CreateElement("Terminal", SclDocument.SCL_XMLNS))
        {

        }
    }

    public class SclTransformerWinding : SclAbstractConductingEquipment
    {
        public string Type
        {
            get
            {
                return "PTW";
            }

        }

        private List<SclTapChanger> sclTapChangers;

        public bool RemoveTapChanger(SclTapChanger node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclTapChangers.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public SclTapChanger AddNewTapChanger()
        {
            SclTapChanger newControl = new SclTapChanger(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (sclTapChangers.Count > 0)
            {
                int lastIndex = sclTapChangers.Count - 1;

                SclTapChanger lastLNode = sclTapChangers[lastIndex];

                XmlNode parent = lastLNode.xmlNode.ParentNode;

                parent.InsertAfter(newNode, lastLNode.xmlNode);
            }
            else
            {
                XmlNode parent = XmlNode;
                parent.AppendChild(newNode);

            }

            try
            {
                newControl = new SclTapChanger(xmlDocument, sclDocument, newNode, nsManager);
                sclTapChangers.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
            }
        }

        private void parseTapChanger()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:TapChanger", nsManager);

            sclTapChangers = new List<SclTapChanger>();

            foreach (XmlNode node in nodes)
            {
                SclTapChanger tap = new SclTapChanger(xmlDocument, sclDocument, node, nsManager);
                sclTapChangers.Add(tap);
            }
        }

        public List<SclTapChanger> TapChangers
        {
            get { return sclTapChangers; }
        }

        private List<SclEqFunction> sclEqFunctions;

        public bool RemoveEqFunction(SclEqFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                EqFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public SclEqFunction AddNewEqFunction()
        {
            SclEqFunction newControl = new SclEqFunction(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (sclEqFunctions.Count > 0)
            {
                int lastIndex = sclEqFunctions.Count - 1;

                SclEqFunction lastLNode = sclEqFunctions[lastIndex];

                XmlNode parent = lastLNode.xmlNode.ParentNode;

                parent.InsertAfter(newNode, lastLNode.xmlNode);
            }
            else
            {
                XmlNode parent = XmlNode;
                parent.AppendChild(newNode);

            }

            try
            {
                newControl = new SclEqFunction(xmlDocument, sclDocument, newNode, nsManager);
                sclEqFunctions.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
            }
        }

        private void parseEqFunctions()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:EqFunction", nsManager);

            sclEqFunctions = new List<SclEqFunction>();

            foreach (XmlNode node in nodes)
            {
                SclEqFunction lNode = new SclEqFunction(xmlDocument, sclDocument, node, nsManager);
                sclEqFunctions.Add(lNode);
            }
        }

        public List<SclEqFunction> EqFunctions
        {
            get { return sclEqFunctions; }
        }


        internal SclTransformerWinding(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            if (Terminals.Count > 1)
            {
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "TransformerWinding contains " + Terminals.Count + " Terminals. Maximum  is 1", this, "Terminal");
            }

            parseTapChanger();

            if (sclTapChangers.Count > 1)
            {
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "TransformerWinding contains " + sclTapChangers.Count + " TapChangers. Maximum  is 1", this, "Terminal");
            }

            parseEqFunctions();
        }

        public SclTransformerWinding(XmlDocument xmlDocument, SclDocument sclDocument, XmlNamespaceManager nsManager)
                : base(xmlDocument, sclDocument, xmlDocument.CreateElement("TransformerWinding", SclDocument.SCL_XMLNS), nsManager)
        {

        }
    }

    public class SclTapChanger : SclPowerSystemResource
    {
        public bool Virtual
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "virtual", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "virtual", value);
            }
        }

        public string Type
        {
            get
            {
                return "LTC";
            }
        }

        private List<SclSubEquipment> sclSubEquipments;

        public bool RemoveSubEquipment(SclSubEquipment node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                SubEquipments.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public SclSubEquipment AddNewSubEquipment()
        {
            SclSubEquipment newControl = new SclSubEquipment(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (SubEquipments.Count > 0)
            {
                int lastIndex = SubEquipments.Count - 1;

                SclSubEquipment lastLNode = SubEquipments[lastIndex];

                XmlNode parent = lastLNode.xmlNode.ParentNode;

                parent.InsertAfter(newNode, lastLNode.xmlNode);
            }
            else
            {
                XmlNode parent = XmlNode;
                parent.AppendChild(newNode);

            }

            try
            {
                newControl = new SclSubEquipment(xmlDocument, sclDocument, newNode, nsManager);
                SubEquipments.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
            }
        }

        private void parseSubEquipment()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:SubEquipment", nsManager);

            sclSubEquipments = new List<SclSubEquipment>();

            foreach (XmlNode node in nodes)
            {
                SclSubEquipment lNode = new SclSubEquipment(xmlDocument, sclDocument, node, nsManager);
                sclSubEquipments.Add(lNode);
            }
        }

        public List<SclSubEquipment> SubEquipments
        {
            get
            {
                return sclSubEquipments;
            }
        }

        public SclEqFunction AddNewEqFunction()
        {
            SclEqFunction newControl = new SclEqFunction(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (sclEqFunctions.Count > 0)
            {
                int lastIndex = sclEqFunctions.Count - 1;

                SclEqFunction lastLNode = sclEqFunctions[lastIndex];

                XmlNode parent = lastLNode.xmlNode.ParentNode;

                parent.InsertAfter(newNode, lastLNode.xmlNode);
            }
            else
            {
                XmlNode parent = XmlNode;
                parent.AppendChild(newNode);

            }

            try
            {
                newControl = new SclEqFunction(xmlDocument, sclDocument, newNode, nsManager);
                sclEqFunctions.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
            }
        }

        public bool RemoveEqFunction(SclEqFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                EqFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }


        private List<SclEqFunction> sclEqFunctions;

        private void parseEqFunctions()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:EqFunction", nsManager);

            sclEqFunctions = new List<SclEqFunction>();

            foreach (XmlNode node in nodes)
            {
                SclEqFunction lNode = new SclEqFunction(xmlDocument, sclDocument, node, nsManager);
                sclEqFunctions.Add(lNode);
            }
        }

        public List<SclEqFunction> EqFunctions
        {
            get { return sclEqFunctions; }
        }


        internal SclTapChanger(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseSubEquipment();

            parseEqFunctions();
        }

        public SclTapChanger(XmlDocument xmlDocument, SclDocument sclDocument, XmlNamespaceManager nsManager)
               : base(xmlDocument, sclDocument, xmlDocument.CreateElement("TapChanger", SclDocument.SCL_XMLNS), nsManager)
        {

        }
    }

}
