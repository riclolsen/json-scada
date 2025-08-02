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
    public class SclAbstractEqFuncSubFunc : SclPowerSystemResource
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

        private List<SclEqSubFunction> sclEqSubFunctions;
        private List<SclGeneralEquipment> sclGeneralEquipments;

        public bool RemoveEqSubFunction(SclEqSubFunction node)
        {
            try
            {
                XmlNode parent = node.xmlNode.ParentNode;

                if (parent != null)
                {
                    parent.RemoveChild(node.xmlNode);
                }

                sclEqSubFunctions.Remove(node);

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

        private void parseEqSubFunction()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:EqSubFunction", nsManager);

            sclEqSubFunctions = new List<SclEqSubFunction>();

            foreach (XmlNode node in nodes)
            {
                SclEqSubFunction lNode = new SclEqSubFunction(xmlDocument, sclDocument, node, nsManager);
                sclEqSubFunctions.Add(lNode);
            }
        }

        private void parseGeneralEquipment()
        {
            XmlNodeList nodes = xmlNode.SelectNodes("scl:GeneralEquipment", nsManager);

            sclGeneralEquipments = new List<SclGeneralEquipment>();

            foreach (XmlNode node in nodes)
            {
                SclGeneralEquipment lNode = new SclGeneralEquipment(xmlDocument, sclDocument, node, nsManager);
                sclGeneralEquipments.Add(lNode);
            }
        }

        public List<SclEqSubFunction> EqSubFunctions
        {
            get { return sclEqSubFunctions; }
        }

        public SclEqSubFunction AddNewEqSubFunction()
        {
            SclEqSubFunction newgeneralEquipment = new SclEqSubFunction(xmlDocument, sclDocument, nsManager);

            XmlNode newNode = newgeneralEquipment.xmlNode;

            if (newNode.OwnerDocument != xmlDocument)
            {
                newNode = xmlDocument.ImportNode(newgeneralEquipment.xmlNode.CloneNode(true), true);
            }

            if (EqSubFunctions.Count > 0)
            {
                int lastIndex = EqSubFunctions.Count - 1;

                SclEqSubFunction lastLNode = EqSubFunctions[lastIndex];

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
                newgeneralEquipment = new SclEqSubFunction(xmlDocument, sclDocument, newNode, nsManager);
                EqSubFunctions.Add(newgeneralEquipment);

                return newgeneralEquipment;

            }
            catch (SclParserException e)
            {
                Console.WriteLine("Failed to add Substation");
                Console.WriteLine(e.ToString());

                return null;
            }
        }


        public List<SclGeneralEquipment> GeneralEquipments
        {
            get { return sclGeneralEquipments; }
        }

        internal SclAbstractEqFuncSubFunc(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
           : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
            parseEqSubFunction();

            parseGeneralEquipment();
        }
    }

    public class SclEqSubFunction : SclAbstractEqFuncSubFunc
    {
        internal SclEqSubFunction(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
           : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
        }

        public SclEqSubFunction(XmlDocument xmlDocument, SclDocument sclDocument, XmlNamespaceManager nsManager)
               : base(xmlDocument, sclDocument, xmlDocument.CreateElement("EqSubFunction", SclDocument.SCL_XMLNS), nsManager)
        {

        }
    }

    public class SclEqFunction : SclAbstractEqFuncSubFunc
    {
        internal SclEqFunction(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
           : base(xmlDocument, sclDocument, xmlNode, nsManager)
        {
        }

        public SclEqFunction(XmlDocument xmlDocument, SclDocument sclDocument, XmlNamespaceManager nsManager)
               : base(xmlDocument, sclDocument, xmlDocument.CreateElement("EqFunction", SclDocument.SCL_XMLNS), nsManager)
        {

        }
    }
}
