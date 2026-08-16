from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field
import json
from .. import types

from ..types import UNSET, Unset

from ..models.evidence_side import EvidenceSide
from ..types import File, FileTypes
from ..types import UNSET, Unset
from io import BytesIO


T = TypeVar("T", bound="NewEvidenceRequest")


@_attrs_define
class NewEvidenceRequest:
    file: File
    """ The evidence file. """
    side: EvidenceSide
    """ Which side uploaded this evidence. """
    caption: str | Unset = UNSET
    """ Optional caption for the evidence. """

    def to_dict(self) -> dict[str, Any]:
        file = self.file.to_tuple()

        side = self.side.value

        caption = self.caption

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "file": file,
                "side": side,
            }
        )
        if caption is not UNSET:
            field_dict["caption"] = caption

        return field_dict

    def to_multipart(self) -> types.RequestFiles:
        files: types.RequestFiles = []

        files.append(("file", self.file.to_tuple()))

        files.append(("side", (None, str(self.side.value).encode(), "text/plain")))

        if not isinstance(self.caption, Unset):
            files.append(("caption", (None, str(self.caption).encode(), "text/plain")))

        return files

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        file = File(payload=BytesIO(d.pop("file")))

        side = EvidenceSide(d.pop("side"))

        caption = d.pop("caption", UNSET)

        new_evidence_request = cls(
            file=file,
            side=side,
            caption=caption,
        )

        return new_evidence_request
