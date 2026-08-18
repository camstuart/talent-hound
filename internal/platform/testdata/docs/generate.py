#!/usr/bin/env python3
"""Regenerate the synthetic extraction fixtures (just fixtures).

Synthetic data only — no real candidate information ever enters this repo.
Each fixture carries MARKER so the sidecar gate can assert on extracted text.
"""
import os
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
MARKER = "TALENTHOUND-FIXTURE-MARKER"
TEXT = MARKER + " synthetic resume"
# Structure the golden gate asserts survives extraction, plus non-ASCII text
# that proves the encoding survives with it. All invented.
UNICODE = "Café Ǆ 東京 — naïve résumé"
HEADING = "Synthetic Experience"
BULLET = "Ran a synthetic pipeline"
CELL_A, CELL_B = "Skill", "Years"


def pdf(path, text):
    objs = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "
        b"/Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
    ]
    stream = ("BT /F1 18 Tf 72 700 Td (%s) Tj ET" % text).encode()
    objs.append(b"<< /Length %d >>\nstream\n" % len(stream) + stream + b"\nendstream")
    objs.append(b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

    out, offsets = b"%PDF-1.4\n", []
    for i, obj in enumerate(objs, 1):
        offsets.append(len(out))
        out += b"%d 0 obj\n" % i + obj + b"\nendobj\n"
    xref = len(out)
    out += b"xref\n0 %d\n" % (len(objs) + 1) + b"0000000000 65535 f \n"
    for off in offsets:
        out += b"%010d 00000 n \n" % off
    out += b"trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n" % (
        len(objs) + 1,
        xref,
    )
    with open(path, "wb") as fh:
        fh.write(out)


def docx(path, text):
    types = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
        '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
        '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
        '<Default Extension="xml" ContentType="application/xml"/>'
        '<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-'
        'officedocument.wordprocessingml.document.main+xml"/></Types>'
    )
    rels = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/'
        'relationships/officeDocument" Target="word/document.xml"/></Relationships>'
    )
    def para(runs, style=None):
        props = '<w:pPr><w:pStyle w:val="%s"/></w:pPr>' % style if style else ""
        return "<w:p>%s<w:r><w:t xml:space=\"preserve\">%s</w:t></w:r></w:p>" % (props, runs)

    def cell(runs):
        return "<w:tc><w:tcPr/>%s</w:tc>" % para(runs)

    body = (
        para(text)
        + para(HEADING, "Heading1")
        + para(BULLET, "ListParagraph")
        + para(UNICODE)
        + "<w:tbl><w:tblPr/><w:tblGrid/>"
        + "<w:tr>%s%s</w:tr>" % (cell(CELL_A), cell(CELL_B))
        + "<w:tr>%s%s</w:tr>" % (cell("Go"), cell("7"))
        + "</w:tbl>"
    )
    doc = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
        '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
        "<w:body>%s</w:body></w:document>" % body
    )
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("[Content_Types].xml", types)
        z.writestr("_rels/.rels", rels)
        z.writestr("word/document.xml", doc)


if __name__ == "__main__":
    pdf(os.path.join(HERE, "fixture.pdf"), TEXT)
    docx(os.path.join(HERE, "fixture.docx"), TEXT)
    with open(os.path.join(HERE, "corrupt.pdf"), "wb") as fh:
        fh.write(b"%PDF-1.4\nnot actually a pdf\n")
    print("fixtures written to", HERE)
