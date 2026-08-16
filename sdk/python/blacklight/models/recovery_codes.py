from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
import datetime


T = TypeVar("T", bound="RecoveryCodes")


@_attrs_define
class RecoveryCodes:
    """A freshly minted set of recovery codes (M1-007), in the only response
    that ever carries them. Put them in front of the person immediately and
    offer a way to save or print them: there is no endpoint that reads them
    back, because the server stores only their hashes.

    """

    codes: list[str]
    """ Ten single-use codes, each twenty characters of Crockford base32 —
    the digits and the uppercase letters, less `I`, `L`, `O` and `U`, so
    that no two characters in a code look alike. That is 100 bits each.

    They arrive grouped in fours for transcription. The grouping is
    presentation: the server accepts them back in any case, with or
    without the hyphens, and folds the four omitted characters onto
    what they resemble (`O`→`0`, `I`/`L`→`1`) so that somebody's
    handwriting cannot lock them out.
     """
    generated_at: datetime.datetime
    """ When this set was minted. It is what an interface shows next to
    "you last replaced these on …", and the moment every previous code
    stopped working.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        codes = self.codes

        generated_at = self.generated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "codes": codes,
                "generatedAt": generated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        codes = cast(list[str], d.pop("codes"))

        generated_at = datetime.datetime.fromisoformat(d.pop("generatedAt"))

        recovery_codes = cls(
            codes=codes,
            generated_at=generated_at,
        )

        recovery_codes.additional_properties = d
        return recovery_codes

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
