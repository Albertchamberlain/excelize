// Copyright 2016 - 2023 The excelize Authors. All rights reserved. Use of
// this source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Package excelize providing a set of functions that allow you to write to and
// read from XLAM / XLSM / XLSX / XLTM / XLTX files. Supports reading and
// writing spreadsheet documents generated by Microsoft Excel™ 2007 and later.
// Supports complex components by high compatibility, and provided streaming
// API for generating or reading data from a worksheet with huge amounts of
// data. This library needs Go version 1.16 or later.

package excelize

import (
	"bytes"
	"encoding/xml"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// SetWorkbookProps provides a function to sets workbook properties.
// SetWorkbookProps 用于设置工作簿属性
func (f *File) SetWorkbookProps(opts *WorkbookPropsOptions) error {
	wb, err := f.workbookReader()
	if err != nil {
		return err
	}
	if wb.WorkbookPr == nil {
		wb.WorkbookPr = new(xlsxWorkbookPr)
	}
	if opts == nil {
		return nil
	}
	if opts.Date1904 != nil {
		wb.WorkbookPr.Date1904 = *opts.Date1904
	}
	if opts.FilterPrivacy != nil {
		wb.WorkbookPr.FilterPrivacy = *opts.FilterPrivacy
	}
	if opts.CodeName != nil {
		wb.WorkbookPr.CodeName = *opts.CodeName
	}
	return nil
}

// GetWorkbookProps provides a function to gets workbook properties.
// GetWorkbookProps 用于获取工作簿属性
func (f *File) GetWorkbookProps() (WorkbookPropsOptions, error) {
	var opts WorkbookPropsOptions
	wb, err := f.workbookReader()
	if err != nil {
		return opts, err
	}
	if wb.WorkbookPr != nil {
		opts.Date1904 = boolPtr(wb.WorkbookPr.Date1904)
		opts.FilterPrivacy = boolPtr(wb.WorkbookPr.FilterPrivacy)
		opts.CodeName = stringPtr(wb.WorkbookPr.CodeName)
	}
	return opts, err
}

// ProtectWorkbook provides a function to prevent other users from viewing
// hidden worksheets, adding, moving, deleting, or hiding worksheets, and
// renaming worksheets in a workbook. The optional field AlgorithmName
// specified hash algorithm, support XOR, MD4, MD5, SHA-1, SHA2-56, SHA-384,
// and SHA-512 currently, if no hash algorithm specified, will be using the XOR
// algorithm as default. The generated workbook only works on Microsoft Office
// 2007 and later. For example, protect workbook with protection settings:
//
//	err := f.ProtectWorkbook(&excelize.WorkbookProtectionOptions{
//	    Password:      "password",
//	    LockStructure: true,
//	})
//
// 使用密码保护工作簿的结构，以防止其他用户查看隐藏的工作表，添加、移动或隐藏工作表以及重命名工作表。
// 字段 AlgorithmName 支持指定哈希算法 XOR、MD4、MD5、SHA-1、SHA-256、SHA-384 或 SHA-512，如果未指定哈希算法，默认使用 XOR 算法。
func (f *File) ProtectWorkbook(opts *WorkbookProtectionOptions) error {
	wb, err := f.workbookReader()
	if err != nil {
		return err
	}
	if wb.WorkbookProtection == nil {
		wb.WorkbookProtection = new(xlsxWorkbookProtection)
	}
	if opts == nil {
		opts = &WorkbookProtectionOptions{}
	}
	wb.WorkbookProtection = &xlsxWorkbookProtection{
		LockStructure: opts.LockStructure,
		LockWindows:   opts.LockWindows,
	}
	if opts.Password != "" {
		if opts.AlgorithmName == "" {
			opts.AlgorithmName = "SHA-512"
		}
		hashValue, saltValue, err := genISOPasswdHash(opts.Password, opts.AlgorithmName, "", int(workbookProtectionSpinCount))
		if err != nil {
			return err
		}
		wb.WorkbookProtection.WorkbookAlgorithmName = opts.AlgorithmName
		wb.WorkbookProtection.WorkbookSaltValue = saltValue
		wb.WorkbookProtection.WorkbookHashValue = hashValue
		wb.WorkbookProtection.WorkbookSpinCount = int(workbookProtectionSpinCount)
	}
	return nil
}

// UnprotectWorkbook provides a function to remove protection for workbook,
// specified the optional password parameter to remove workbook protection with
// password verification.
// 取保护工作簿，指定可选密码参数以通过密码验证来取消工作簿保护。
func (f *File) UnprotectWorkbook(password ...string) error {
	wb, err := f.workbookReader()
	if err != nil {
		return err
	}
	// password verification
	if len(password) > 0 {
		if wb.WorkbookProtection == nil {
			return ErrUnprotectWorkbook
		}
		if wb.WorkbookProtection.WorkbookAlgorithmName != "" {
			// check with given salt value
			hashValue, _, err := genISOPasswdHash(password[0], wb.WorkbookProtection.WorkbookAlgorithmName, wb.WorkbookProtection.WorkbookSaltValue, wb.WorkbookProtection.WorkbookSpinCount)
			if err != nil {
				return err
			}
			if wb.WorkbookProtection.WorkbookHashValue != hashValue {
				return ErrUnprotectWorkbookPassword
			}
		}
	}
	wb.WorkbookProtection = nil
	return err
}

// setWorkbook update workbook property of the spreadsheet. Maximum 31
// characters are allowed in sheet title.
func (f *File) setWorkbook(name string, sheetID, rid int) {
	wb, _ := f.workbookReader()
	wb.Sheets.Sheet = append(wb.Sheets.Sheet, xlsxSheet{
		Name:    name,
		SheetID: sheetID,
		ID:      "rId" + strconv.Itoa(rid),
	})
}

// getWorkbookPath provides a function to get the path of the workbook.xml in
// the spreadsheet.
// 获取book地址
func (f *File) getWorkbookPath() (path string) {
	if rels, _ := f.relsReader("_rels/.rels"); rels != nil {
		rels.mu.Lock()
		defer rels.mu.Unlock()
		for _, rel := range rels.Relationships {
			if rel.Type == SourceRelationshipOfficeDocument {
				path = strings.TrimPrefix(rel.Target, "/")
				return
			}
		}
	}
	return
}

// getWorkbookRelsPath provides a function to get the path of the workbook.xml.rels
// in the spreadsheet.
// 获取book相对地址
func (f *File) getWorkbookRelsPath() (path string) {
	wbPath := f.getWorkbookPath()
	wbDir := filepath.Dir(wbPath)
	if wbDir == "." {
		path = "_rels/" + filepath.Base(wbPath) + ".rels"
		return
	}
	path = strings.TrimPrefix(filepath.Dir(wbPath)+"/_rels/"+filepath.Base(wbPath)+".rels", "/")
	return
}

// workbookReader provides a function to get the pointer to the workbook.xml
// structure after deserialization.
func (f *File) workbookReader() (*xlsxWorkbook, error) {
	var err error
	if f.WorkBook == nil {
		wbPath := f.getWorkbookPath()
		f.WorkBook = new(xlsxWorkbook)
		if _, ok := f.xmlAttr[wbPath]; !ok {
			d := f.xmlNewDecoder(bytes.NewReader(namespaceStrictToTransitional(f.readXML(wbPath))))
			f.xmlAttr[wbPath] = append(f.xmlAttr[wbPath], getRootElement(d)...)
			f.addNameSpaces(wbPath, SourceRelationship)
		}
		if err = f.xmlNewDecoder(bytes.NewReader(namespaceStrictToTransitional(f.readXML(wbPath)))).
			Decode(f.WorkBook); err != nil && err != io.EOF {
			return f.WorkBook, err
		}
	}
	return f.WorkBook, err
}

// workBookWriter provides a function to save workbook.xml after serialize
// structure.
func (f *File) workBookWriter() {
	if f.WorkBook != nil {
		if f.WorkBook.DecodeAlternateContent != nil {
			f.WorkBook.AlternateContent = &xlsxAlternateContent{
				Content: f.WorkBook.DecodeAlternateContent.Content,
				XMLNSMC: SourceRelationshipCompatibility.Value,
			}
		}
		f.WorkBook.DecodeAlternateContent = nil
		output, _ := xml.Marshal(f.WorkBook)
		f.saveFileList(f.getWorkbookPath(), replaceRelationshipsBytes(f.replaceNameSpaceBytes(f.getWorkbookPath(), output)))
	}
}
