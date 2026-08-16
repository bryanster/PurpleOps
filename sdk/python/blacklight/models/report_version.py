from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ReportVersion")


@_attrs_define
class ReportVersion:
    id: UUID
    report_id: UUID
    ordinal: int
    title: str
    published_by: str
    published_at: datetime.datetime
    include_evidence: bool
    blind_scope: str
    """ Always 'lead_full' for published versions. """
    content_sha_256: None | str | Unset = UNSET
    pdf_sha_256: None | str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        report_id = str(self.report_id)

        ordinal = self.ordinal

        title = self.title

        published_by = self.published_by

        published_at = self.published_at.isoformat()

        include_evidence = self.include_evidence

        blind_scope = self.blind_scope

        content_sha_256: None | str | Unset
        if isinstance(self.content_sha_256, Unset):
            content_sha_256 = UNSET
        else:
            content_sha_256 = self.content_sha_256

        pdf_sha_256: None | str | Unset
        if isinstance(self.pdf_sha_256, Unset):
            pdf_sha_256 = UNSET
        else:
            pdf_sha_256 = self.pdf_sha_256

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "id": id,
                "reportId": report_id,
                "ordinal": ordinal,
                "title": title,
                "publishedBy": published_by,
                "publishedAt": published_at,
                "includeEvidence": include_evidence,
                "blindScope": blind_scope,
            }
        )
        if content_sha_256 is not UNSET:
            field_dict["contentSha256"] = content_sha_256
        if pdf_sha_256 is not UNSET:
            field_dict["pdfSha256"] = pdf_sha_256

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        report_id = UUID(d.pop("reportId"))

        ordinal = d.pop("ordinal")

        title = d.pop("title")

        published_by = d.pop("publishedBy")

        published_at = datetime.datetime.fromisoformat(d.pop("publishedAt"))

        include_evidence = d.pop("includeEvidence")

        blind_scope = d.pop("blindScope")

        def _parse_content_sha_256(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        content_sha_256 = _parse_content_sha_256(d.pop("contentSha256", UNSET))

        def _parse_pdf_sha_256(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        pdf_sha_256 = _parse_pdf_sha_256(d.pop("pdfSha256", UNSET))

        report_version = cls(
            id=id,
            report_id=report_id,
            ordinal=ordinal,
            title=title,
            published_by=published_by,
            published_at=published_at,
            include_evidence=include_evidence,
            blind_scope=blind_scope,
            content_sha_256=content_sha_256,
            pdf_sha_256=pdf_sha_256,
        )

        return report_version
