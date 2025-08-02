/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System.Collections.Generic;
using System.Xml;

namespace IEC61850.SCL
{
    public class SclDataTypeTemplates : SclElementWithPrivate
    {
        private List<SclLNodeType> lnTypes = new List<SclLNodeType>();
        private List<SclDOType> doTypes = new List<SclDOType>();
        private List<SclDAType> daTypes = new List<SclDAType>();
        private List<SclEnumType> enumTypes = new List<SclEnumType>();


        internal SclDataTypeTemplates(SclDocument sclDocument, XmlNode xmlNode)
                 : base(sclDocument, xmlNode)
        {
            foreach (XmlNode lNodeType in xmlNode.SelectNodes("scl:LNodeType", sclDocument.NsManager))
            {
                SclLNodeType newType = new SclLNodeType(sclDocument, lNodeType);

                if (newType != null)
                    lnTypes.Add(newType);
            }

            foreach (XmlNode doType in xmlNode.SelectNodes("scl:DOType", sclDocument.NsManager))
            {
                SclDOType newType = new SclDOType(sclDocument, doType);

                if (newType != null)
                    doTypes.Add(newType);
            }

            foreach (XmlNode daType in xmlNode.SelectNodes("scl:DAType", sclDocument.NsManager))
            {
                SclDAType newType = new SclDAType(sclDocument, daType);

                if (newType != null)
                    daTypes.Add(newType);
            }

            foreach (XmlNode enumType in xmlNode.SelectNodes("scl:EnumType", sclDocument.NsManager))
            {
                SclEnumType newType = new SclEnumType(sclDocument, enumType);

                if (newType != null)
                    enumTypes.Add(newType);
            }
        }

        /// <summary>
        /// Add a new type to the DataTypeTemplates section. The type will be placed at the correct position
	    /// according to the class.
        /// </summary>
        /// <param name="sclType">the new type to add</param>
        public SclType AddType(SclType sclType)
        {
            if (sclType.xmlNode.OwnerDocument != sclDocument.XmlDocument)
            {
                XmlNode importedNode = sclDocument.XmlDocument.ImportNode(sclType.xmlNode, true);
                sclType.xmlNode = importedNode;
            }

            if (sclType is SclLNodeType)
            {
                if (doTypes.Count > 0)
                    xmlNode.InsertBefore(sclType.xmlNode, doTypes[0].xmlNode);
                else if (daTypes.Count > 0)
                    xmlNode.InsertBefore(sclType.xmlNode, daTypes[0].xmlNode);
                else if (enumTypes.Count > 0)
                    xmlNode.InsertBefore(sclType.xmlNode, enumTypes[0].xmlNode);
                else
                    xmlNode.InsertBefore(sclType.xmlNode, null);

                lnTypes.Add(sclType as SclLNodeType);
            }
            else if (sclType is SclDOType)
            {
                if (daTypes.Count > 0)
                    xmlNode.InsertBefore(sclType.xmlNode, daTypes[0].xmlNode);
                else if (enumTypes.Count > 0)
                    xmlNode.InsertBefore(sclType.xmlNode, enumTypes[0].xmlNode);
                else
                    xmlNode.InsertBefore(sclType.xmlNode, null);

                doTypes.Add(sclType as SclDOType);
            }
            else if (sclType is SclDAType)
            {
                if (enumTypes.Count > 0)
                    xmlNode.InsertBefore(sclType.xmlNode, enumTypes[0].xmlNode);
                else
                    xmlNode.InsertBefore(sclType.xmlNode, null);

                daTypes.Add(sclType as SclDAType);
            }
            else if (sclType is SclEnumType)
            {
                xmlNode.InsertBefore(sclType.xmlNode, null);

                enumTypes.Add(sclType as SclEnumType);
            }

            return sclType;
        }



        /// <summary>
        /// Remove a type from the DataTypeTemplates section.
        /// </summary>
        /// <param name="type">the type to remove</param>
        public void RemoveType(SclType type)
        {
            if (type is SclLNodeType)
                lnTypes.Remove(type as SclLNodeType);
            else if (type is SclDOType)
                doTypes.Remove(type as SclDOType);
            else if (type is SclDAType)
                daTypes.Remove(type as SclDAType);
            else if (type is SclEnumType)
                enumTypes.Remove(type as SclEnumType);

            if (type.xmlNode.ParentNode != null)
                type.xmlNode.ParentNode.RemoveChild(type.xmlNode);
        }

        public void RemoveIfUnused(SclType sclType)
        {
            if (sclType.IsUsed == false)
            {
                RemoveType(sclType);
            }
        }

        /// <summary>
        /// Remove all unused types from the DataTypeTemplates section
        /// </summary>
        public void RemoveUnusedTypes()
        {
            List<SclType> types = new List<SclType>();
            types.AddRange(LNTypes);
            types.AddRange(DOTypes);
            types.AddRange(DATypes);
            types.AddRange(EnumTypes);

            foreach (SclType sclType in types)
                RemoveIfUnused(sclType);

        }

        public SclLNodeType GetLNodeType(string id)
        {
            foreach (SclLNodeType type in lnTypes)
            {
                if (type.Id.Equals(id))
                    return type;
            }

            return null;
        }

        public SclDOType GetDOType(string id)
        {
            foreach (SclDOType type in doTypes)
            {
                if (type.Id.Equals(id))
                    return type;
            }

            return null;
        }

        public SclDAType GetDAType(string id)
        {
            foreach (SclDAType type in daTypes)
            {
                if (type.Id.Equals(id))
                    return type;
            }

            return null;
        }

        public SclEnumType GetEnumType(string id)
        {
            foreach (SclEnumType type in enumTypes)
            {
                if (type.Id.Equals(id))
                    return type;
            }

            return null;
        }

        public List<SclLNodeType> LNTypes
        {
            get
            {
                return new List<SclLNodeType>(lnTypes);
            }
        }

        public List<SclDOType> DOTypes
        {
            get
            {
                return new List<SclDOType>(doTypes);
            }
        }

        public List<SclDAType> DATypes
        {
            get
            {
                return new List<SclDAType>(daTypes);
            }
        }

        public List<SclEnumType> EnumTypes
        {
            get
            {
                return new List<SclEnumType>(enumTypes);
            }
        }

        public List<SclType> AllTypes
        {
            get
            {
                List<SclType> types = new List<SclType>();

                foreach (SclType type in lnTypes)
                    types.Add(type);

                foreach (SclType type in doTypes)
                    types.Add(type);

                foreach (SclType type in daTypes)
                    types.Add(type);

                foreach (SclType type in enumTypes)
                    types.Add(type);

                return types;
            }
        }

    }
}
