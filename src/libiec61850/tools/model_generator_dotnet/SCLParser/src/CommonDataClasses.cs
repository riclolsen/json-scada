/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System.Collections.Generic;

namespace IEC61850.SCL
{
    public enum CDCAttributeTrgOp
    {
        NONE,
        DCHG,
        QCHG,
        DUPD
    }

    public enum CDCAttributeOptionality
    {
        M,
        O,
        PICS_SUBST,
        AC_DLNDA_M,
        AC_DLN_M
    }

    public class CDCAttribute
    {
        private string name;
        private AttributeType bType;
        private string type = null;
        private SclFC fc;
        CDCAttributeTrgOp trgOp;
        CDCAttributeOptionality optionality;

        public string Name
        {
            get
            {
                return name;
            }
        }

        public AttributeType BType
        {
            get
            {
                return bType;
            }
        }

        public string Type
        {
            get
            {
                return type;
            }
        }

        public SclFC Fc
        {
            get
            {
                return fc;
            }
        }

        public CDCAttributeTrgOp TrgOp
        {
            get
            {
                return trgOp;
            }
        }

        public CDCAttribute(string name, AttributeType bType, string type, SclFC fc, CDCAttributeTrgOp trgOp, CDCAttributeOptionality optionality)
        {
            this.name = name;
            this.bType = bType;
            this.type = type;
            this.fc = fc;
            this.trgOp = trgOp;
            this.optionality = optionality;
        }

    }

    public class CommonDataClass
    {
        private string name;

        public string Name
        {
            get
            {
                return name;
            }
        }

        public List<CDCAttribute> Attributes
        {
            get
            {
                return attributes;
            }
        }

        List<CDCAttribute> attributes = new List<CDCAttribute>();

        public CommonDataClass(string name, CDCAttribute[] attributes)
        {
            this.name = name;

            foreach (CDCAttribute cdcAttribute in attributes)
                this.attributes.Add(cdcAttribute);
        }
    }

    public class StandardCommonDataClasses
    {
        private List<CommonDataClass> cdcs = new List<CommonDataClass>();

        public StandardCommonDataClasses()
        {
            cdcs.Add(new CommonDataClass("SPS", new CDCAttribute[] {
                    new CDCAttribute("stVal", AttributeType.BOOLEAN, null, SclFC.ST, CDCAttributeTrgOp.DCHG, CDCAttributeOptionality.M),
                     new CDCAttribute("q", AttributeType.QUALITY, null, SclFC.ST, CDCAttributeTrgOp.QCHG, CDCAttributeOptionality.M),
                     new CDCAttribute("t", AttributeType.TIMESTAMP, null, SclFC.ST, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.M),

                    new CDCAttribute("subEna", AttributeType.BOOLEAN, null, SclFC.SV, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.PICS_SUBST),
                    new CDCAttribute("subVal", AttributeType.BOOLEAN, null, SclFC.SV, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.PICS_SUBST),
                    new CDCAttribute("subQ", AttributeType.QUALITY, null, SclFC.SV, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.PICS_SUBST),
                    new CDCAttribute("subID", AttributeType.VISIBLE_STRING_64, null, SclFC.SV, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.PICS_SUBST),
                    new CDCAttribute("blkEna", AttributeType.BOOLEAN, null, SclFC.BL, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.O),

                    new CDCAttribute("d", AttributeType.VISIBLE_STRING_255, null, SclFC.DC, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.O),
                    new CDCAttribute("dU", AttributeType.UNICODE_STRING_255, null, SclFC.DC, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.O),

                    new CDCAttribute("cdcNs", AttributeType.VISIBLE_STRING_255, null, SclFC.EX, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.AC_DLNDA_M),
                    new CDCAttribute("cdcName", AttributeType.VISIBLE_STRING_255, null, SclFC.EX, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.AC_DLNDA_M),
                    new CDCAttribute("dataNs", AttributeType.VISIBLE_STRING_255, null, SclFC.EX, CDCAttributeTrgOp.NONE, CDCAttributeOptionality.AC_DLN_M)
                }));
        }


        public CommonDataClass GetByName(string cdcName)
        {
            foreach (CommonDataClass cdc in cdcs)
            {
                if (cdc.Name.Equals(cdcName))
                    return cdc;
            }

            return null;
        }
    }
}

