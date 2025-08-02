/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System;
using System.Collections.Generic;
using System.Text;
using System.Xml;

namespace IEC61850.SCL
{
    public class SclLNode : SclBaseElement
    {
        private XmlDocument xmlDocument;

        public string IedName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "iedName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "iedName", value);
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

        public SclLNode(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode)
        {
            this.xmlNode = xmlNode;
            this.xmlDocument = xmlDocument;

            if (LdInst == null)
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "ldInst is missing in LNode element!", this, "LdInst");

            if (LnClass == null)
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "lnClass is missing in LNode element!", this, "LnClass");

        }


        public string GetObjectReference()
        {
            StringBuilder objRef = new StringBuilder();

            if (IedName != null)
                objRef.Append(IedName);

            objRef.Append(LdInst);
            objRef.Append('/');

            if (Prefix != null)
                objRef.Append(Prefix);

            if (LnClass != null)
                objRef.Append(LnClass);

            if (LnInst != null)
                objRef.Append(LnInst);

            return objRef.ToString();
        }
    }

    public class SclLNodeContainer : SclBaseElement
    {
        protected XmlDocument xmlDocument;
        protected XmlNamespaceManager nsManager;

        private List<SclLNode> lNodes;

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

        private void parseLNodes()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:LNode", nsManager);

            lNodes = new List<SclLNode>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclLNode lNode = new SclLNode(xmlDocument, sclDocument, lNodeNode, nsManager);
                lNodes.Add(lNode);
            }
        }


        internal SclLNodeContainer(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
            : base(xmlDocument, sclDocument, xmlNode)
        {
            this.xmlNode = xmlNode;
            this.xmlDocument = xmlDocument;
            this.nsManager = nsManager;

            parseLNodes();
        }

        public List<SclLNode> LNodes
        {
            get
            {
                return lNodes;
            }
        }
    }

    public class SclConnectivityNode : SclLNodeContainer
    {
        public string PathName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "pathName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "pathName", value);
            }
        }

        internal SclConnectivityNode(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {

        }

    }

    public class SclPowerSystemResource : SclLNodeContainer
    {
        internal SclPowerSystemResource(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
                 : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {

        }
    }

    public class SclSubFunction : SclPowerSystemResource
    {
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

        private List<SclSubFunction> sclSubFunctions;
        private List<SclGeneralEquipment> sclGeneralEquipments;
        private List<SclConductingEquipment> sclConductingEquipments;

        public bool RemoveSubFunction(SclSubFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclSubFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }


        public bool RemoveGeneralEquipment(SclGeneralEquipment node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclGeneralEquipments.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public bool RemoveConductingEquipment(SclConductingEquipment node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclConductingEquipments.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        private void parseSubFunction()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:SubFunction", nsManager);

            sclSubFunctions = new List<SclSubFunction>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclSubFunction subFunction = new SclSubFunction(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclSubFunctions.Add(subFunction);
            }
        }

        private void parseGeneralEquipment()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:GeneralEquipment", nsManager);

            sclGeneralEquipments = new List<SclGeneralEquipment>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclGeneralEquipment generalEquipment = new SclGeneralEquipment(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclGeneralEquipments.Add(generalEquipment);
            }
        }

        private void parseConductingEquipment()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:ConductingEquipment", nsManager);

            sclConductingEquipments = new List<SclConductingEquipment>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclConductingEquipment conductingEquipment = new SclConductingEquipment(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclConductingEquipments.Add(conductingEquipment);
            }
        }

        public List<SclConductingEquipment> ConductingEquipments
        {
            get
            {
                return sclConductingEquipments;
            }
        }

        public List<SclGeneralEquipment> GeneralEquipments
        {
            get
            {
                return sclGeneralEquipments;
            }
        }

        public List<SclSubFunction> SubFunctions
        {
            get
            {
                return sclSubFunctions;
            }
        }

        internal SclSubFunction(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseSubFunction();

            parseGeneralEquipment();

            parseConductingEquipment();
        }


    }

    public class SclFunction : SclPowerSystemResource
    {
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

        private List<SclSubFunction> sclSubFunctions;
        private List<SclGeneralEquipment> sclGeneralEquipments;
        private List<SclConductingEquipment> sclConductingEquipments;

        public bool RemoveSubFunction(SclSubFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclSubFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public bool RemoveGeneralEquipment(SclGeneralEquipment node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclGeneralEquipments.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        private void parseSubFunction()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:SubFunction", nsManager);

            sclSubFunctions = new List<SclSubFunction>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclSubFunction subFunction = new SclSubFunction(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclSubFunctions.Add(subFunction);
            }
        }

        private void parseGeneralEquipment()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:GeneralEquipment", nsManager);

            sclGeneralEquipments = new List<SclGeneralEquipment>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclGeneralEquipment generalEquipment = new SclGeneralEquipment(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclGeneralEquipments.Add(generalEquipment);
            }
        }

        private void parseConductingEquipment()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:ConductingEquipment", nsManager);

            sclConductingEquipments = new List<SclConductingEquipment>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclConductingEquipment conductingEquipment = new SclConductingEquipment(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclConductingEquipments.Add(conductingEquipment);
            }
        }

        public List<SclConductingEquipment> ConductingEquipments
        {
            get
            {
                return sclConductingEquipments;
            }
        }

        public List<SclGeneralEquipment> GeneralEquipments
        {
            get
            {
                return sclGeneralEquipments;
            }
        }

        public List<SclSubFunction> SubFunctions
        {
            get
            {
                return sclSubFunctions;
            }
        }

        internal SclFunction(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseGeneralEquipment();

            parseSubFunction();

            parseConductingEquipment();
        }


    }

    public class SclEquipmentContainer : SclPowerSystemResource
    {
        private List<SclPowerTransformer> sclPowerTransformers;

        public bool RemovePowerTransformer(SclPowerTransformer node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclPowerTransformers.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public List<SclPowerTransformer> PowerTransformers
        {
            get
            {
                return sclPowerTransformers;
            }
        }

        private void parsePowerTransformer()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:PowerTransformer", nsManager);

            sclPowerTransformers = new List<SclPowerTransformer>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclPowerTransformer element = new SclPowerTransformer(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclPowerTransformers.Add(element);
            }
        }

        private List<SclGeneralEquipment> sclGeneralEquipments;

        public List<SclGeneralEquipment> GeneralEquipments
        {
            get
            {
                return sclGeneralEquipments;
            }
        }

        private void parseGeneralEquipment()
        {
            XmlNodeList lNodeNodes = xmlNode.SelectNodes("scl:GeneralEquipment", nsManager);

            sclGeneralEquipments = new List<SclGeneralEquipment>();

            foreach (XmlNode lNodeNode in lNodeNodes)
            {
                SclGeneralEquipment element = new SclGeneralEquipment(xmlDocument, sclDocument, lNodeNode, nsManager);
                sclGeneralEquipments.Add(element);
            }
        }


        internal SclEquipmentContainer(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
                 : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parsePowerTransformer();

            parseGeneralEquipment();
        }
    }

    public class SclSubstation : SclEquipmentContainer
    {
        private List<SclVoltageLevel> voltageLevels = new List<SclVoltageLevel>();
        private List<SclFunction> sclFunctions = new List<SclFunction>();

        public bool RemoveVoltageLevel(SclVoltageLevel node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                voltageLevels.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public bool RemoveFunction(SclFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        private void ParseVoltageLevels()
        {
            XmlNodeList voltageLevelNodes = xmlNode.SelectNodes("scl:VoltageLevel", nsManager);

            voltageLevels = new List<SclVoltageLevel>();

            foreach (XmlNode voltageLevelNode in voltageLevelNodes)
            {
                SclVoltageLevel voltageLevel = new SclVoltageLevel(xmlDocument, sclDocument, voltageLevelNode, nsManager);

                voltageLevels.Add(voltageLevel);
            }
        }

        private void ParseFunction()
        {
            XmlNodeList functionlNodes = xmlNode.SelectNodes("scl:Function", nsManager);

            sclFunctions = new List<SclFunction>();

            foreach (XmlNode functionlNode in functionlNodes)
            {
                SclFunction function = new SclFunction(xmlDocument, sclDocument, functionlNode, nsManager);

                sclFunctions.Add(function);
            }
        }

        public List<SclVoltageLevel> VoltageLevels
        {
            get
            {
                return voltageLevels;
            }
        }

        public List<SclFunction> Functions
        {
            get
            {
                return sclFunctions;
            }
        }

        internal SclSubstation(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
                 : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {

            ParseVoltageLevels();

            if (voltageLevels.Count < 1)
            {
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "Substation contains no Voltage Level. Minimum is 1", this, "VoltageLevel");
            }

            ParseFunction();
        }


    }

    public class SclVoltageLevel : SclEquipmentContainer
    {
        private List<SclBay> sclBays;
        private List<SclFunction> sclFunctions;
        private List<SclVoltage> sclVoltages;

        public bool RemoveVoltage(SclVoltage node)
        {
            try
            {
                XmlNode parent = node.XmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.XmlNode);
                }

                sclVoltages.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }


        public bool RemoveFunction(SclFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public bool RemoveBay(SclBay node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                Bays.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        private void ParseVoltage()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:Voltage", nsManager);

            sclVoltages = new List<SclVoltage>();

            foreach (XmlNode node in nodes)
            {
                SclVoltage voltage = new SclVoltage(xmlDocument, sclDocument, node, nsManager);

                sclVoltages.Add(voltage);
            }
        }
        private void ParseBay()
        {
            XmlNodeList Nodes = xmlNode.SelectNodes("scl:Bay", nsManager);

            sclBays = new List<SclBay>();

            foreach (XmlNode Node in Nodes)
            {
                SclBay function = new SclBay(xmlDocument, sclDocument, Node, nsManager);

                sclBays.Add(function);
            }
        }
        private void ParseFunction()
        {
            XmlNodeList functionlNodes = xmlNode.SelectNodes("scl:Function", nsManager);

            sclFunctions = new List<SclFunction>();

            foreach (XmlNode functionlNode in functionlNodes)
            {
                SclFunction function = new SclFunction(xmlDocument, sclDocument, functionlNode, nsManager);

                sclFunctions.Add(function);
            }
        }

        public List<SclBay> Bays
        {
            get
            {
                return sclBays;
            }
        }

        public List<SclFunction> Functions
        {
            get
            {
                return sclFunctions;
            }
        }

        public List<SclVoltage> Voltages
        {
            get
            {
                return sclVoltages;
            }
        }

        internal SclVoltageLevel(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            ParseVoltage();

            ParseBay();

            if (sclBays.Count < 1)
            {
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "VoltageLevel contains no Bay. Minimum is 1", this, "Bay");

            }

            ParseFunction();
        }

    }

    public class SclBay : SclEquipmentContainer
    {
        private List<SclConnectivityNode> sclConnectivityNodes;
        private List<SclFunction> sclFunctions;
        private List<SclConductingEquipment> sclConductingEquipments;

        public bool RemoveFunction(SclFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclFunctions.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        private void ParseConnectivityNode()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:ConnectivityNode", nsManager);

            sclConnectivityNodes = new List<SclConnectivityNode>();

            foreach (XmlNode node in nodes)
            {
                SclConnectivityNode connectivityNode = new SclConnectivityNode(xmlDocument, sclDocument, node, nsManager);

                sclConnectivityNodes.Add(connectivityNode);
            }
        }

        private void ParseConductingEquipment()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:ConductingEquipment", nsManager);

            sclConductingEquipments = new List<SclConductingEquipment>();

            foreach (XmlNode node in nodes)
            {
                SclConductingEquipment conductingEquipment = new SclConductingEquipment(xmlDocument, sclDocument, node, nsManager);

                sclConductingEquipments.Add(conductingEquipment);
            }
        }

        private void ParseFunction()
        {
            XmlNodeList functionlNodes = xmlNode.SelectNodes("scl:Function", nsManager);

            sclFunctions = new List<SclFunction>();

            foreach (XmlNode functionlNode in functionlNodes)
            {
                SclFunction function = new SclFunction(xmlDocument, sclDocument, functionlNode, nsManager);

                sclFunctions.Add(function);
            }
        }

        public List<SclConnectivityNode> ConnectivityNodes
        {
            get
            {
                return sclConnectivityNodes;
            }
        }

        public List<SclFunction> Functions
        {
            get
            {
                return sclFunctions;
            }
        }

        public List<SclConductingEquipment> ConductingEquipments
        {
            get
            {
                return sclConductingEquipments;
            }
        }
        internal SclBay(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            ParseConnectivityNode();

            ParseFunction();

            ParseConductingEquipment();
        }


    }

    public class SclVoltage
    {
        public XmlNode XmlNode;
        XmlDocument XmlDocument;
        public string Type
        {
            get
            {
                return XmlHelper.GetAttributeValue(XmlNode, "type");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(XmlDocument, XmlNode, "type", value);
            }
        }

        internal SclVoltage(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            XmlNode = xmlNode;
            XmlDocument = xmlDocument;
        }

    }

    public class SclPowerTransformer : SclEquipment
    {
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

        private List<SclSubEquipment> sclSubEquipments;
        private List<SclTransformerWinding> sclTransformerwindings;
        private List<SclEqFunction> sclEqFunctions;

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

        private void parseTransformerWinding()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:TransformerWinding", nsManager);

            sclTransformerwindings = new List<SclTransformerWinding>();

            foreach (XmlNode node in nodes)
            {
                SclTransformerWinding lNode = new SclTransformerWinding(xmlDocument, sclDocument, node, nsManager);
                sclTransformerwindings.Add(lNode);
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

        public List<SclSubEquipment> SubEquipments
        {
            get
            {
                return sclSubEquipments;
            }
        }

        public List<SclTransformerWinding> Transformerwindings
        {
            get
            {
                return sclTransformerwindings;
            }
        }

        public List<SclEqFunction> EqFunctions
        {
            get { return sclEqFunctions; }
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

        public bool RemoveTransformerWinding(SclTransformerWinding node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                Transformerwindings.Remove(node);

                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        public SclTransformerWinding AddNewTransformerWinding()
        {
            SclTransformerWinding newControl = new SclTransformerWinding(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newControl.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newControl.xmlNode.CloneNode(true), true);
            }

            if (Transformerwindings.Count > 0)
            {
                int lastIndex = Transformerwindings.Count - 1;

                SclTransformerWinding lastLNode = Transformerwindings[lastIndex];

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
                newControl = new SclTransformerWinding(xmlDocument, sclDocument, newNode, nsManager);
                Transformerwindings.Add(newControl);

                return newControl;

            }
            catch (SclParserException e)
            {
                Console.WriteLine(e.ToString());

                return null;
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

        internal SclPowerTransformer(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseSubEquipment();

            parseTransformerWinding();

            if (sclTransformerwindings.Count < 1)
            {
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "PowerTransformer contains no TransformerWinding. Minimum is 1", this, "BTransformerWindingay");

            }

            parseEqFunctions();
        }
    }

    public class SclGeneralEquipment : SclEquipment
    {
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

        private List<SclEqFunction> sclEqFunctions;

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

        private void parceEqFunctions()
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

        internal SclGeneralEquipment(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parceEqFunctions();
        }

    }

    public class SclConductingEquipment : SclAbstractConductingEquipment
    {
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

        private List<SclEqFunction> sclEqFunctions;

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


        internal SclConductingEquipment(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
         : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseEqFunctions();
        }

    }

}
