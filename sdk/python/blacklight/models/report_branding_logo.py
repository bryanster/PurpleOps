from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="ReportBrandingLogo")


@_attrs_define
class ReportBrandingLogo:
    """Response from uploading a branding logo."""

    blob_ref: str
    """ Content-addressed blob reference for the uploaded logo. """

    def to_dict(self) -> dict[str, Any]:
        blob_ref = self.blob_ref

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "blobRef": blob_ref,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        blob_ref = d.pop("blobRef")

        report_branding_logo = cls(
            blob_ref=blob_ref,
        )

        return report_branding_logo
