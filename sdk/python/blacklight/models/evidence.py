from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.evidence_side import EvidenceSide
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="Evidence")


@_attrs_define
class Evidence:
    id: UUID
    blob_sha_256: str
    """ SHA-256 hex digest of the blob content. """
    filename: str
    caption: str
    side: EvidenceSide
    """ Which side uploaded this evidence. """
    execution_id: UUID
    """ The execution this evidence is linked to. """
    uploaded_by: str
    """ User id of the uploader. """
    uploaded_at: datetime.datetime
    size: int
    """ File size in bytes. """
    mime: str
    """ Stored MIME type from upload. """
    comment_id: None | Unset | UUID = UNSET
    """ The comment this evidence is linked to (null when execution-only). """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        blob_sha_256 = self.blob_sha_256

        filename = self.filename

        caption = self.caption

        side = self.side.value

        execution_id = str(self.execution_id)

        uploaded_by = self.uploaded_by

        uploaded_at = self.uploaded_at.isoformat()

        size = self.size

        mime = self.mime

        comment_id: None | str | Unset
        if isinstance(self.comment_id, Unset):
            comment_id = UNSET
        elif isinstance(self.comment_id, UUID):
            comment_id = str(self.comment_id)
        else:
            comment_id = self.comment_id

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "blobSha256": blob_sha_256,
                "filename": filename,
                "caption": caption,
                "side": side,
                "executionId": execution_id,
                "uploadedBy": uploaded_by,
                "uploadedAt": uploaded_at,
                "size": size,
                "mime": mime,
            }
        )
        if comment_id is not UNSET:
            field_dict["commentId"] = comment_id

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        blob_sha_256 = d.pop("blobSha256")

        filename = d.pop("filename")

        caption = d.pop("caption")

        side = EvidenceSide(d.pop("side"))

        execution_id = UUID(d.pop("executionId"))

        uploaded_by = d.pop("uploadedBy")

        uploaded_at = datetime.datetime.fromisoformat(d.pop("uploadedAt"))

        size = d.pop("size")

        mime = d.pop("mime")

        def _parse_comment_id(data: object) -> None | Unset | UUID:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                comment_id_type_0 = UUID(data)

                return comment_id_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(None | Unset | UUID, data)

        comment_id = _parse_comment_id(d.pop("commentId", UNSET))

        evidence = cls(
            id=id,
            blob_sha_256=blob_sha_256,
            filename=filename,
            caption=caption,
            side=side,
            execution_id=execution_id,
            uploaded_by=uploaded_by,
            uploaded_at=uploaded_at,
            size=size,
            mime=mime,
            comment_id=comment_id,
        )

        evidence.additional_properties = d
        return evidence

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
