from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field
import json
from .. import types

from ..types import UNSET, Unset

from ..models.import_custom_content_request_format import ImportCustomContentRequestFormat
from ..types import File, FileTypes
from ..types import UNSET, Unset
from io import BytesIO


T = TypeVar("T", bound="ImportCustomContentRequest")


@_attrs_define
class ImportCustomContentRequest:
    """Multipart v1 custom import. `file` is a single JSON/YAML document or a
    zip of files; `format` selects the parser (or `auto` to sniff).

    """

    file: File
    """ Upload bytes. Size is capped by `BLACKLIGHT_CONTENT_MAX_BYTES`
    (default 512 MiB).
     """
    format_: ImportCustomContentRequestFormat | Unset = ImportCustomContentRequestFormat.AUTO
    """ Parser selection. Defaults to `auto`.
     """

    def to_dict(self) -> dict[str, Any]:
        file = self.file.to_tuple()

        format_: str | Unset = UNSET
        if not isinstance(self.format_, Unset):
            format_ = self.format_.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "file": file,
            }
        )
        if format_ is not UNSET:
            field_dict["format"] = format_

        return field_dict

    def to_multipart(self) -> types.RequestFiles:
        files: types.RequestFiles = []

        files.append(("file", self.file.to_tuple()))

        if not isinstance(self.format_, Unset):
            files.append(("format", (None, str(self.format_.value).encode(), "text/plain")))

        return files

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        file = File(payload=BytesIO(d.pop("file")))

        _format_ = d.pop("format", UNSET)
        format_: ImportCustomContentRequestFormat | Unset
        if isinstance(_format_, Unset):
            format_ = UNSET
        else:
            format_ = ImportCustomContentRequestFormat(_format_)

        import_custom_content_request = cls(
            file=file,
            format_=format_,
        )

        return import_custom_content_request
