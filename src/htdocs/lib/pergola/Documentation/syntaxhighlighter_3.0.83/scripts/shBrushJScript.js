/**
 * SyntaxHighlighter
 * http://alexgorbatchev.com/SyntaxHighlighter
 *
 * SyntaxHighlighter is donationware. If you are using it, please donate.
 * http://alexgorbatchev.com/SyntaxHighlighter/donate.html
 *
 * @version
 * 3.0.83 (July 02 2010)
 * 
 * @copyright
 * Copyright (C) 2004-2010 Alex Gorbatchev.
 *
 * @license
 * Dual licensed under the MIT and GPL licenses.
 */
;(function()
{
	// CommonJS
	typeof(require) != 'undefined' ? SyntaxHighlighter = require('shCore').SyntaxHighlighter : null;

	function Brush()
	{
		var keywords =	'break case catch continue ' +
						'default delete do else false  ' +
						'for function if in instanceof ' +
						'new null return super switch ' +
						'this throw true try typeof var while with',
				keywords1 =	'abs acos alert apply arguments Array asin atan atan2 call ceil charAt charCode charCodeAt clientX clientY concat concat confirm constructor content cos ' +
				'currentTarget data Date detail E Equal event every evt exec exp filter floor forEach fromCharCode Function getDate getDay getFullYear getHours getMilliseconds getMinutes ' +
				'getMonth getSeconds getTime getTimezoneOffset getURL getUTCDate getUTCDay getUTCFullYear getUTCHours getUTCMilliseconds getUTCMinutes getUTCMonth getUTCSeconds getYear ' +
				'global hasFeature keyCode ignoreCase implementation index indexOf isSupported join LN10 LN2 LOG10E LOG2E lastIndex lastIndexOf length localeCompare localName log Math map ' +
				'match max MAX_VALUE min MIN_VALUE multiline MutationEvent NaN namespaceURI normalize NEGATIVE_INFINITY Number Object open parseFloat parseFromString parseInt PI pop ' +
				'POSITIVE_INFINITY pow prefix print prompt prototype push random reduce reduceRight replace reverse round search send setDate setFullYear setHours setMilliseconds setMinutes ' +
				'setMonth setSeconds setTime setUTCDate setUTCFullYear setUTCHours setUTCMilliseconds setUTCMinutes setUTCMonth setUTCSeconds setYear shift sin slice some sort source splice ' +
				'split SQRT1_2 SQRT2 sqrt String substr substring success tan target test toDateString toExponential toFixed toGMTString toLocaleDateString toLocaleFormat toLocaleLowerCase ' +
				'toPrecision toSource toString toTimeString toUpperCase toUTCString type unshift valueOftoLocaleString toLocaleTimeString toLocaleUpperCase toLowerCase',
				keywordsDOM =	'addEventListener appendChild attributes childNodes cloneNode createElementNS createTextNode document documentElement DOMParser firstChild getAttribute ' +
				'getAttributeNode getAttributeNodeNS getAttributeNS getBBox getCTM getElementById getElementsByTagName getElementsByTagNameNS getResponseHeader handleEvent hasAttribute ' +
				'hasAttributeNS hasAttributes hasChildNodes importNode indexOf insertBefore lastChild lastIndexOf namespaceURI nextSibling nodeName nodeType nodeValue normalize onreadystatechange ' +
				'open ownerDocument parent parentNode parseXML preventDefault previousSibling readyState responseText responseXML removeAttribute removeAttributeNode removeAttributeNodeNS ' +
				'removeEventListener replaceChild send setAttribute setAttributeNode setAttributeNodeNS setAttributeNS stopPropagation window XMLHttpRequestremoveAttributeNS removeChild',
				numbers = /[0-9]/gm,
//			. , : ; + - * / = % < > ! | & ~ ( ) { } [ ]
				operators = /['\u002E''\u002C''\u003A''\u003B''\u002B''\u002D''\u002A''\u002F''\u003D''\u0025''\u003C''\u003E''\u0021''\u007C''\u0026''\u007E''\u0028''\u0029''\u007B''\u007D''\u005B''\u005D']/gm;

		var r = SyntaxHighlighter.regexLib;
		
		this.regexList = [
			{ regex: r.multiLineDoubleQuotedString,					css: 'string' },			// double quoted strings
			{ regex: r.multiLineSingleQuotedString,					css: 'string' },			// single quoted strings
			{ regex: r.singleLineCComments,							css: 'comments' },			// one line comments
			{ regex: r.multiLineCComments,							css: 'comments' },			// multiline comments
			{ regex: /\s*#.*/gm,									css: 'preprocessor' },		// preprocessor tags like #region and #endregion
			{ regex: new RegExp(this.getKeywords(keywords), 'gm'),		css: 'keyword' },				// keywords
			{ regex: new RegExp(this.getKeywords(keywords1), 'gm'),		css: 'keyword1' },			// keywords1
			{ regex: new RegExp(this.getKeywords(keywordsDOM), 'gm'),	css: 'keywordDOM' },		// keywords DOM
			{ regex: numbers,																					css: 'number' },				// numbers
			{ regex: operators,																				css: 'operator' }				// operators
		];
	
		this.forHtmlScript(r.scriptScriptTags);
	};

	Brush.prototype	= new SyntaxHighlighter.Highlighter();
	Brush.aliases	= ['js', 'jscript', 'javascript'];

	SyntaxHighlighter.brushes.JScript = Brush;

	// CommonJS
	typeof(exports) != 'undefined' ? exports.Brush = Brush : null;
})();
