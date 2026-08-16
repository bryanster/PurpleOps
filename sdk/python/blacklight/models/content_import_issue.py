from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="ContentImportIssue")


@_attrs_define
class ContentImportIssue:
    """One per-file or per-item warning or error from an import."""

    path: str
    """ Source path inside the upload (or `-` for the root file). """
    message: str

    def to_dict(self) -> dict[str, Any]:
        path = self.path

        message = self.message

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "path": path,
                "message": message,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        path = d.pop("path")

        message = d.pop("message")

        content_import_issue = cls(
            path=path,
            message=message,
        )

        return content_import_issue
