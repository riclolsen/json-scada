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
    public class SclCommunication
    {
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;
        public XmlNode xmlNode;
        private SclDocument sclDocument;

        private List<SclSubNetwork> subNetworks = new List<SclSubNetwork>();


        public bool DeleteSubnetwork(SclSubNetwork sclSubNetwork)
        {
            if (subNetworks.Exists(x => x == sclSubNetwork))
            {
                subNetworks.Remove(sclSubNetwork);
                xmlNode.RemoveChild(sclSubNetwork.xmlNode);

                return true;
            }
            else
                return false;
        }

        public SclSubNetwork AddSubNetwork(SclSubNetwork sclSubNetwork)
        {
            XmlNode newNode = sclSubNetwork.xmlNode;

            if (newNode.OwnerDocument != xmlDoc)
            {
                newNode = xmlDoc.ImportNode(sclSubNetwork.xmlNode.CloneNode(true), true);
            }

            if (GetSubNetworks().Count > 0)
                xmlNode.InsertAfter(newNode, GetSubNetworks()[GetSubNetworks().Count - 1].xmlNode);
            else
            {
                xmlNode.AppendChild(newNode);

            }

            try
            {
                SclSubNetwork newSubNetwork = new SclSubNetwork(newNode, sclDocument, nsManager);
                subNetworks.Add(newSubNetwork);

                return newSubNetwork;

            }
            catch (SclParserException ex)
            {
                Console.WriteLine(ex.ToString());
                return null;
            }
        }

        private void ParseSubNetworks()
        {
            if (subNetworks == null)
                subNetworks = new List<SclSubNetwork>();

            XmlNodeList subNetworkNodes = xmlNode.SelectNodes("scl:SubNetwork", nsManager);

            foreach (XmlNode snNode in subNetworkNodes)
            {
                subNetworks.Add(new SclSubNetwork(snNode, sclDocument, nsManager));
            }
        }

        internal SclCommunication(XmlNode comNode, SclDocument SclxmlDoc, XmlNamespaceManager nsManager)
        {
            xmlDoc = SclxmlDoc.XmlDocument;
            xmlNode = comNode;
            this.nsManager = nsManager;
            sclDocument = SclxmlDoc;

            ParseSubNetworks();
        }

        public List<SclSubNetwork> GetSubNetworks()
        {
            if (subNetworks == null)
                ParseSubNetworks();

            return subNetworks;
        }

        public SclConnectedAP GetConnectedAP(string apName, string iedName)
        {
            foreach (SclSubNetwork subNetwork in GetSubNetworks())
            {
                foreach (SclConnectedAP connectedAp in subNetwork.GetConnectedAPs())
                {
                    if (connectedAp.ApName.Equals(apName) && connectedAp.IedName.Equals(iedName))
                        return connectedAp;
                }
            }

            return null;
        }

        public SclSubNetwork GetSubNetwork(string apName, string iedName)
        {
            foreach (SclSubNetwork subNetwork in GetSubNetworks())
            {
                foreach (SclConnectedAP connectedAp in subNetwork.GetConnectedAPs())
                {
                    if (connectedAp.ApName.Equals(apName) && connectedAp.IedName.Equals(iedName))
                        return subNetwork;
                }
            }

            return null;
        }
    }

    public class SclSubNetwork
    {
        private XmlDocument xmlDoc;
        internal XmlNode xmlNode;
        private XmlNamespaceManager nsManager;
        private SclDocument sclDocument;

        public XmlNode XmlNodeClone()
        {
            XmlNode newNode = xmlNode.CloneNode(true);

            return newNode;
        }

        private XmlNamespaceManager NsManager
        {
            get { return nsManager; }
        }

        private List<SclConnectedAP> connectedAPs = null;

        private void ParseConnectedAPs()
        {
            if (connectedAPs == null)
                connectedAPs = new List<SclConnectedAP>();

            if (connectedAPs.Count > 0)
                connectedAPs.Clear();

            XmlNodeList connectedAPNodes = xmlNode.SelectNodes("scl:ConnectedAP", nsManager);

            foreach (XmlNode node in connectedAPNodes)
            {
                connectedAPs.Add(new SclConnectedAP(node, sclDocument, nsManager));
            }
        }

        public SclSubNetwork(XmlNode snNode, SclDocument SclxmlDoc, XmlNamespaceManager nsManager)
        {
            xmlNode = snNode;
            xmlDoc = SclxmlDoc.XmlDocument;
            sclDocument = SclxmlDoc;
            this.nsManager = nsManager;
            XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "Name", Name);

            if (Name == null)
                SclxmlDoc.AddIssue(snNode, "ERROR", "Model integrity", "SubNetwork has no name attribute", this, "MissingName");

            ParseConnectedAPs();
        }


        public List<SclConnectedAP> GetConnectedAPs()
        {
            if (connectedAPs == null)
                ParseConnectedAPs();

            return connectedAPs;
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

        public string Type
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "type");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "type", value);
            }
        }
    }

    public class SclConnectedAP
    {
        private SclDocument sclDocument;
        private XmlDocument xmlDoc;
        internal XmlNode xmlNode;
        private SclAddress address = null;
        private XmlNamespaceManager nsManager;
        private List<SclGSE> gses = new List<SclGSE>();
        private List<SclSMV> smvs = new List<SclSMV>();

        public XmlNode XmlNodeClone()
        {
            XmlNode newNode = xmlNode.CloneNode(true);

            return newNode;
        }

        private XmlNamespaceManager NsManager
        {
            get { return nsManager; }
        }

        public SclConnectedAP(XmlNode apNode, SclDocument SclxmlDoc, XmlNamespaceManager nsManager)
        {
            sclDocument = SclxmlDoc;
            xmlNode = apNode;
            xmlDoc = SclxmlDoc.XmlDocument;
            this.nsManager = nsManager;

            if (IedName == null)
                SclxmlDoc.AddIssue(xmlNode, "ERROR", "Model integrity", "ConnectedAP is missing required attribute iedName", this, "connectedAP");

            //throw new SclParserException(apNode, "ConnectedAP is missing required attribute.");

            if (ApName == null)
                SclxmlDoc.AddIssue(xmlNode, "ERROR", "Model integrity", "ConnectedAP is missing required attribute apName", this, "connectedAP");


            XmlNode AddressNode = xmlNode.SelectSingleNode("scl:Address", nsManager);
            if (AddressNode != null)
            {
                address = new SclAddress(AddressNode, SclxmlDoc, nsManager);
            }

            XmlNodeList GSENodes = xmlNode.SelectNodes("scl:GSE", nsManager);

            foreach (XmlNode node in GSENodes)
            {
                gses.Add(new SclGSE(node, SclxmlDoc, nsManager));
            }

            XmlNodeList SMVNodes = xmlNode.SelectNodes("scl:SMV", nsManager);

            foreach (XmlNode node in SMVNodes)
            {
                smvs.Add(new SclSMV(node, SclxmlDoc, nsManager));
            }
        }


        public List<SclGSE> GSEs
        {
            get { return gses; }
            set { gses = value; }

        }

        public List<SclSMV> SMVs
        {
            get { return smvs; }
            set { smvs = value; }

        }

        public SclAddress Address
        {
            get { return address; }
            set { address = value; }

        }

        public SclAddress GetAddress()
        {
            if (address != null)
                return address;

            XmlNode addrNode = xmlNode.SelectSingleNode("scl:Address", nsManager);

            if (addrNode != null)
            {
                address = new SclAddress(addrNode, sclDocument, nsManager);

                return address;
            }
            else
                return null;
        }

        public string IedName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "iedName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "iedName", value);
            }
        }

        public string ApName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "apName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "apName", value);
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

        public string RedProt
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "redProt");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, xmlNode, "redProt", value);
            }
        }
    }

    public class SclGSE
    {
        public XmlNode gseNode;
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;
        private SclAddress sclAddress = null;
        private XmlNode minTime = null;
        private XmlNode maxTime = null;

        public XmlNode Node { get { return gseNode; } set { gseNode = value; } }
        public SclAddress SclAddress { get { return sclAddress; } set { sclAddress = value; } }

        public SclGSE(XmlNode GSENode, SclDocument SclxmlDoc, XmlNamespaceManager nsManager)
        {
            gseNode = GSENode;
            xmlDoc = SclxmlDoc.XmlDocument;
            this.nsManager = nsManager;

            XmlAttribute nameAttr = gseNode.Attributes["cbName"];
            if (nameAttr == null)
                SclxmlDoc.AddIssue(GSENode, "ERROR", "Model integrity", "GSE has no cbName attribute", this, "MissingName");

            XmlNode AddressNode = gseNode.SelectSingleNode("scl:Address", nsManager);
            if (AddressNode != null)
            {
                sclAddress = new SclAddress(AddressNode, SclxmlDoc, nsManager);
            }

            XmlNode min = gseNode.SelectSingleNode("scl:MinTime", nsManager);
            if (min != null)
            {
                minTime = min;
            }

            XmlNode max = gseNode.SelectSingleNode("scl:MaxTime", nsManager);
            if (max != null)
            {
                maxTime = max;
            }

        }

        public string Mintime
        {
            get
            {
                if (minTime != null)
                {
                    return minTime.InnerText;
                }
                else
                    return null;
            }
        }

        public string Maxtime
        {
            get
            {
                if (maxTime != null)
                {
                    return maxTime.InnerText;
                }
                else
                    return null;
            }
        }

        public string CbName
        {
            get
            {
                return XmlHelper.GetAttributeValue(gseNode, "cbName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, gseNode, "cbName", value);
            }
        }

        public string LdInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(gseNode, "ldInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, gseNode, "ldInst", value);
            }
        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(gseNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, gseNode, "desc", value);
            }
        }
    }

    public class SclSMV
    {
        public XmlNode smvNode;
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;
        private SclAddress sclAddress = null;

        public SclAddress SclAddress { get { return sclAddress; } set { sclAddress = value; } }

        public SclSMV(XmlNode SMVNode, SclDocument SclxmlDoc, XmlNamespaceManager nsManager)
        {
            smvNode = SMVNode;
            xmlDoc = SclxmlDoc.XmlDocument;
            this.nsManager = nsManager;

            XmlAttribute nameAttr = smvNode.Attributes["cbName"];
            if (nameAttr == null)
                SclxmlDoc.AddIssue(SMVNode, "ERROR", "Model integrity", "SMV has no cbName attribute", this, "MissingName");

            XmlNode AddressNode = smvNode.SelectSingleNode("scl:Address", nsManager);
            if (AddressNode != null)
            {
                sclAddress = new SclAddress(AddressNode, SclxmlDoc, nsManager);
            }


        }

        public string CbName
        {
            get
            {
                return XmlHelper.GetAttributeValue(smvNode, "cbName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, smvNode, "cbName", value);
            }
        }

        public string LdInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(smvNode, "ldInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, smvNode, "ldInst", value);
            }
        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(smvNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, smvNode, "desc", value);
            }
        }
    }

    public class SclAddress
    {
        public XmlNode addrNode;
        private XmlDocument xmlDoc;
        private XmlNamespaceManager nsManager;

        public XmlNode Node { get { return addrNode; } set { addrNode = value; } }
        private string vlanId = "0";
        private string vlanPriority = "0";
        private string appId = "0";
        private int[] macAddress = null;

        public string VlanId
        {
            get
            {
                return vlanId;
            }
            set
            {
                vlanId = value;
            }

        }

        public string VlanPriority
        {
            get
            {
                return vlanPriority;
            }
            set
            {
                vlanPriority = value;
            }

        }

        public string AppId
        {
            get
            {
                return appId;
            }
            set
            {
                appId = value;
            }

        }

        public int[] MacAddress
        {
            get
            {
                return macAddress;

            }
            set
            {
                macAddress = value;
            }
        }

        private List<SclP> sclPs = new List<SclP>();

        public List<SclP> SclPs { get { return sclPs; } set { sclPs = value; } }

        public void CheckIfPValueisCorrect(SclDocument sclDocument, SclP sclP)
        {
            if (sclP.Text != null)
            {
                if (sclP.Type == "APPID")
                {
                    if (System.Text.RegularExpressions.Regex.IsMatch(sclP.Text, @"\A\b[0-9a-fA-F]+\b\Z"))
                    {
                        if (sclP.Text.Length != 4)
                        {
                            sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " size it is not 4. Actual size: " + sclP.Text.Length.ToString(), sclP, "value");

                        }
                    }
                    else
                    {
                        sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " does not have a Hex value. Value is: " + sclP.Text, sclP, "value");

                    }

                }
                else if (sclP.Type == "VLAN-ID")
                {
                    if (System.Text.RegularExpressions.Regex.IsMatch(sclP.Text, @"\A\b[0-9a-fA-F]+\b\Z"))
                    {
                        if (sclP.Text.Length != 3)
                        {
                            sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " size it is not 3. Actual size: " + sclP.Text.Length.ToString(), sclP, "value");

                        }
                    }
                    else
                    {
                        sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " does not have a Hex value. Value is: " + sclP.Text, sclP, "value");

                    }

                }
                else if (sclP.Type == "VLAN-PRIORITY")
                {
                    if (System.Text.RegularExpressions.Regex.IsMatch(sclP.Text, @"\A\b[0-7]+\b\Z"))
                    {
                    }
                    else
                    {
                        sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " does not have a 0-7 value. Value is: " + sclP.Text, sclP, "value");

                    }
                }
                else if (sclP.Type == "MAC-Address")
                {
                    string[] addressElements = sclP.Text.Split('-');

                    if (addressElements.Length == 6)
                    {
                        for (int i = 0; i < addressElements.Length; i++)
                        {
                            if (System.Text.RegularExpressions.Regex.IsMatch(addressElements[i], @"\A\b[0-9a-fA-F]+\b\Z"))
                            {
                                if (addressElements[i].Length != 2)
                                    sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " size it is not 2 on position " + i + ". Actual size: " + addressElements[i].Length.ToString(), sclP, "value");

                            }
                            else
                            {
                                sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " does not have a Hex value on position " + i + ". Value is: " + addressElements[i], sclP, "value");

                            }
                        }
                    }
                    else
                        sclDocument.AddIssue(sclP.Node, "ERROR", "Model integrity", "P address of type " + sclP.Type + " does not have a array size 6 value. Actual size: " + addressElements.Length.ToString(), sclP, "value");

                }

            }

        }

        public SclAddress(XmlNode addrNode, SclDocument sclDocument, XmlNamespaceManager nsManager)
        {
            this.addrNode = addrNode;
            xmlDoc = sclDocument.XmlDocument;
            this.nsManager = nsManager;

            XmlNodeList pNodes = addrNode.SelectNodes("scl:P", nsManager);

            foreach (XmlNode pNode in pNodes)
            {
                SclP sclP = new SclP(pNode, xmlDoc);
                sclPs.Add(sclP);

                CheckIfPValueisCorrect(sclDocument, sclP);

                if (sclP.Text != null)
                {
                    if (sclP.Type == "VLAN-ID")
                        VlanId = sclP.Text;
                    else if (sclP.Type == "VLAN-PRIORITY")
                        VlanPriority = sclP.Text;
                    else if (sclP.Type == "APPID")
                        AppId = sclP.Text;
                    else if (sclP.Type == "MAC-Address")
                    {
                        string[] addressElements = sclP.Text.Split('-');

                        if (addressElements.Length == 6)
                        {
                            macAddress = new int[6];

                            for (int i = 0; i < addressElements.Length; i++)
                            {
                                macAddress[i] = int.Parse(addressElements[i], System.Globalization.NumberStyles.AllowHexSpecifier);
                            }
                        }

                    }



                }


                else if (sclP.Type == "MAC-Address")
                {
                    string[] addressElements = sclP.Text.Split('-');

                    if (addressElements.Length == 6)
                    {
                        macAddress = new int[6];

                        for (int i = 0; i < addressElements.Length; i++)
                        {
                            macAddress[i] = int.Parse(addressElements[i], System.Globalization.NumberStyles.AllowHexSpecifier);
                        }
                    }

                }
            }
        }

        public List<SclP> GetAddressParameters()
        {
            if (sclPs == null)
            {
                sclPs = new List<SclP>();

                XmlNodeList pNodes = addrNode.SelectNodes("scl:P", nsManager);

                foreach (XmlNode pNode in pNodes)
                {
                    sclPs.Add(new SclP(pNode, xmlDoc));
                }
            }

            return sclPs;
        }

        public SclP GetAddressParameter(string type)
        {
            foreach (SclP p in GetAddressParameters())
            {
                if (p.Type != null)
                {
                    if (p.Type.Equals(type))
                        return p;
                }
            }

            return null;
        }
    }

    public class SclP
    {
        private XmlNode pNode;
        private XmlDocument xmlDoc;

        internal XmlNode Node { get { return pNode; } set { pNode = value; } }

        public SclP(XmlNode pNode, XmlDocument xmlDoc)
        {
            this.pNode = pNode;
            this.xmlDoc = xmlDoc;
        }

        public XmlNode XmlNode
        {
            get { return pNode; }
        }

        public string Type
        {
            get
            {
                return XmlHelper.GetAttributeValue(pNode, "type");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, pNode, "type", value);
            }
        }

        public string XsiType
        {
            get
            {
                return XmlHelper.GetAttributeValue(pNode, "xsi:type");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDoc, pNode, "xsi:type", value);
            }
        }

        public string Text
        {
            get
            {
                return pNode.InnerText;
            }

            set
            {
                pNode.InnerText = value;
            }
        }
    }

}
